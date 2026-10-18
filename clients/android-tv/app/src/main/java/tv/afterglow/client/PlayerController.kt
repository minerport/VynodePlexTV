package tv.afterglow.client

import android.content.Context
import androidx.media3.common.MediaItem
import androidx.media3.common.Player
import androidx.media3.common.PlaybackException
import androidx.media3.datasource.okhttp.OkHttpDataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableStateFlow
import org.json.JSONObject

data class Viewing(val tune:JSONObject?=null,val loading:Boolean=false,val error:String?=null,val pendingChannel:String?=null)
/** One player and one server viewer; a serialized conflated switch prevents overlapping tunes. */
@androidx.annotation.OptIn(androidx.media3.common.util.UnstableApi::class)
class PlayerController(context:Context,private val api:Api,private val scope:CoroutineScope,private val installation:String) {
    val player=ExoPlayer.Builder(context).setMediaSourceFactory(DefaultMediaSourceFactory(OkHttpDataSource.Factory(api.mediaClient))).build()
    val state=MutableStateFlow(Viewing())
    var capabilities=Capabilities.detect(context)
    private var pending:Job?=null
    private val mutex=kotlinx.coroutines.sync.Mutex()
    private var generation=0L
    private var heartbeat:Job?=null
    private var metadataStarted=0L
    private var recovery:Job?=null
    private var reconnectAttempts=0
    private var wanted:Triple<String,String,String?>?=null
    init {player.addListener(object:Player.Listener{override fun onPlayerError(error:PlaybackException){android.util.Log.w("AfterglowPlayback", "player error code=${error.errorCode}");state.value=state.value.copy(error="Playback interrupted. Reconnecting…");reconnect()}})}
    fun tune(channel:String,mode:String="LIVE",program:String?=null, recovering:Boolean=false) {
        if(!recovering){reconnectAttempts=0;recovery?.cancel()}
        heartbeat?.cancel()
        wanted=Triple(channel,mode,program);generation++;val epoch=generation
        state.value=state.value.copy(loading=true,error=null,pendingChannel=channel)
        pending?.cancel();pending=scope.launch {
            delay(250)
            mutex.lock()
            try {
                if(epoch!=generation)return@launch
                val old=state.value.tune?.optString("id")
                val c=JSONObject(capabilities.toString()).put("versionId","").put("audioStreamId","").put("subtitleStreamId","")
                val body=JSONObject().put("channelId",channel).put("mode",mode).put("clientId",installation).put("capabilities",c)
                if(!old.isNullOrBlank())body.put("replaceSession",old)
                if(program!=null)body.put("programId",program)
                // Complete negotiation even if a newer key arrives, then dispose its stale result before tuning again.
                val response=withContext(NonCancellable){api.call("/playback/tune","POST",body)}
                if(epoch!=generation){withContext(NonCancellable){api.call("/playback/${response.getString("id")}/stop","POST")};state.value=state.value.copy(tune=null);return@launch}
                apply(response)
            }catch(e:CancellationException){throw e}catch(e:Exception){state.value=state.value.copy(loading=false,error=e.message?:"Unable to tune. Retry playback.")}finally{mutex.unlock()}
        }
    }
    private fun apply(t:JSONObject){
        android.util.Log.i("AfterglowPlayback", "delivery=${t.optString("delivery")} mime=${PlaybackFormat.mime(t.optString("mime"),t.optString("delivery"))}")
        player.stop();player.clearMediaItems();state.value=Viewing(t);metadataStarted=android.os.SystemClock.elapsedRealtime()
        if(t.optString("delivery")!="METADATA_PRESENTATION") {
            player.setMediaItem(MediaItem.Builder().setUri(api.server+t.getString("streamUrl")).setMimeType(PlaybackFormat.mime(t.optString("mime"),t.optString("delivery"))).build())
            player.seekTo(t.optLong("positionMs"));player.prepare();player.play()
        }
        heartbeat?.cancel();heartbeat=scope.launch {while(isActive){val current=state.value.tune?:break
            val remaining=(current.optJSONObject("program")?.optLong("end")?:0)-current.optLong("serverTime")
            delay(if(current.optString("mode")=="LIVE"&&remaining in 1..5000)remaining.coerceAtLeast(100) else 3000)
            try{val response=api.call("/playback/${current.getString("id")}/heartbeat","POST",JSONObject().put("positionMs",if(current.optString("delivery")=="METADATA_PRESENTATION")current.optLong("positionMs")+android.os.SystemClock.elapsedRealtime()-metadataStarted else player.currentPosition.coerceAtLeast(0)).put("state",if(current.optString("delivery")=="METADATA_PRESENTATION"||player.isPlaying)"PLAYING" else if(player.playbackState==Player.STATE_BUFFERING)"STARTING" else "PAUSED").put("ended",player.playbackState==Player.STATE_ENDED))
                current.put("mode",response.optString("mode")).put("serverTime",response.optLong("serverTime")).put("liveOffsetMs",response.optLong("liveOffsetMs"));state.value=state.value.copy(tune=current)
                if(response.optBoolean("transition")){apply(api.call("/playback/${current.getString("id")}/advance","POST"));break}
            }catch(e:Exception){if(e is CancellationException)throw e;state.value=state.value.copy(error="Connection lost. Reconnecting…");reconnect();break}
        }}
    }
    fun pause(){if(player.isPlaying)player.pause() else player.play()}
    fun seek(delta:Long){player.seekTo((player.currentPosition+delta).coerceAtLeast(0))}
    fun goLive(){scope.launch {try{val id=state.value.tune?.getString("id")?:return@launch;apply(api.call("/playback/$id/go-live","POST"));wanted=Triple(state.value.tune!!.getJSONObject("channel").getString("id"),"LIVE",null)}catch(e:Exception){state.value=state.value.copy(error=e.message)}}}
    fun retry(){wanted?.let{tune(it.first,it.second,it.third)}}
    private fun reconnect(){
        if(recovery?.isActive==true)return
        if(reconnectAttempts>=3){heartbeat?.cancel();state.value=state.value.copy(loading=false,error="Playback could not start after 3 attempts. Retry playback or choose another channel.");return}
        val attempt=++reconnectAttempts
        recovery=scope.launch {
            delay(1000L shl (attempt-1))
            val target=wanted?:return@launch
            tune(target.first,target.second,target.third,recovering=true)
        }
    }
    suspend fun options()=api.call("/playback/${state.value.tune?.getString("id")}/options")
    fun option(key:String,value:String){capabilities.put(key,value);scope.launch{try{val id=state.value.tune?.getString("id")?:return@launch;apply(api.call("/playback/$id/options","POST",JSONObject().put("capabilities",capabilities).put("positionMs",player.currentPosition)))}catch(e:Exception){state.value=state.value.copy(error=e.message)}}}
    fun stop(){generation++;pending?.cancel();heartbeat?.cancel();recovery?.cancel();player.stop();val id=state.value.tune?.optString("id");state.value=Viewing();if(!id.isNullOrEmpty())scope.launch {runCatching{api.call("/playback/$id/stop","POST")}}}
    fun release(){stop();player.release()}
}
