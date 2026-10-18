package tv.afterglow.client

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.viewModels
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import android.app.Application
import androidx.compose.runtime.*
import kotlinx.coroutines.*
import org.json.JSONObject
import java.time.Instant

class TvModel(app:Application):AndroidViewModel(app) {
    private val appearancePrefs=app.getSharedPreferences("appearance",android.content.Context.MODE_PRIVATE)
    var themeName by mutableStateOf(appearancePrefs.getString("theme","Tidal")?:"Tidal")
    var motion by mutableStateOf(appearancePrefs.getBoolean("motion",true))
    var compact by mutableStateOf(appearancePrefs.getBoolean("compact",false))
    fun appearance(theme:String=themeName,animate:Boolean=motion,dense:Boolean=compact){
        themeName=theme;motion=animate;compact=dense
        appearancePrefs.edit().putString("theme",theme).putBoolean("motion",animate).putBoolean("compact",dense).apply()
    }
    var remoteHandler:((android.view.KeyEvent)->Boolean)?=null
    val store=SecureStore(app);val engine=EmbeddedEngine(app,store);val api=Api(engine)
    val playback=PlayerController(app,api,viewModelScope,store.installationId)
    var paired by mutableStateOf(false)
    var ready by mutableStateOf(false)
    var pairing by mutableStateOf<JSONObject?>(null)
    var plexChoices by mutableStateOf(listOf<JSONObject>())
    var servers by mutableStateOf(listOf<JSONObject>())
    var libraries by mutableStateOf(listOf<JSONObject>())
    var syncMinutes by mutableIntStateOf(15)
    var jobs by mutableStateOf(listOf<JSONObject>())
    var error by mutableStateOf<String?>(null)
    var busy by mutableStateOf(false)
    var channels by mutableStateOf(listOf<Channel>())
    var ranked by mutableStateOf(listOf<Ranked>())
    var soon by mutableStateOf(listOf<Channel>())
    var guide by mutableStateOf(listOf<Channel>())
    var groups by mutableStateOf(listOf<Pair<String,String>>())
    var last by mutableStateOf("")
    var user by mutableStateOf("Local Viewer")
    var serverName by mutableStateOf("On this device")
    var serverVersion by mutableStateOf(BuildConfig.VERSION_NAME)
    var timezone by mutableStateOf("UTC")
    var serverDelta by mutableLongStateOf(0)
    var guideStart by mutableLongStateOf(System.currentTimeMillis()-30*60000)
    var selectedChannel by mutableStateOf("")
    var selectedProgram by mutableStateOf("")
    private var linking:Job?=null
    private var guideJob:Job?=null
    private var lookupJob:Job?=null
    var guideGroup by mutableStateOf("")
    var guideFavorites by mutableStateOf(false)
    init { start() }
    fun start(){action {engine.start();ready=true;loadSetup();refresh()}}
    fun now()=System.currentTimeMillis()+serverDelta
    private fun action(block:suspend ()->Unit){viewModelScope.launch{busy=true;error=null;try{block()}catch(e:Exception){if(e !is CancellationException)error=e.message}finally{busy=false}}}
    suspend fun loadSetup(){
        syncMinutes=api.call("/plex/settings").optInt("syncIntervalSeconds",900)/60
        servers=api.call("/plex/servers").optJSONArray("items")?.objects()?:emptyList()
        paired=servers.isNotEmpty()
        libraries=servers.flatMap{api.call("/plex/servers/${it.getString("id")}/libraries").optJSONArray("items")?.objects()?:emptyList()}
        val data=api.call("/jobs");jobs=(data.optJSONArray("items")?:data.optJSONArray("jobs"))?.objects()?:emptyList()
    }
    fun setupRefresh(){action{loadSetup()}}
    fun signInPlex(){action{
        linking?.cancel();plexChoices=emptyList();pairing=api.call("/plex/link","POST")
        val link=pairing!!
        linking=viewModelScope.launch {try{while(pairing!=null){delay(2500);val state=api.call("/plex/link/${link.getString("id")}/poll","POST");val result=PlexLinkResult.parse(state);if(result.authorized){plexChoices=result.servers;if(result.servers.isEmpty())error="No Plex media servers are available to this account.";return@launch}}}catch(e:Exception){if(e !is CancellationException)error=e.message}}
    }}
    fun connectPlex(server:JSONObject,uri:String){action{
        val id=pairing?.getString("id")?:return@action
        val saved=api.call("/plex/link/$id/connect","POST",JSONObject().put("serverId",server.getString("id")).put("uri",uri))
        api.call("/plex/servers/${saved.getString("id")}/discover","POST")
        pairing=null;plexChoices=emptyList();loadSetup();refresh()
    }}
    fun selectLibrary(library:JSONObject){action{api.call("/plex/libraries/${library.getString("id")}","PATCH",JSONObject().put("selected",!library.getBoolean("selected")));loadSetup()}}
    fun sync(){action{libraries.filter{it.optBoolean("selected")}.forEach{api.call("/plex/libraries/${it.getString("id")}/sync","POST",JSONObject().put("mode","incremental"))};loadSetup()}}
    var configured by mutableStateOf(listOf<JSONObject>())
    var metadata by mutableStateOf(listOf<JSONObject>())
    var metadataMore by mutableStateOf(false)
    var preview by mutableStateOf<JSONObject?>(null)
    fun loadChannels(){action{configured=api.call("/channels?limit=100").getJSONArray("items").objects()}}
    fun lookup(field:String,query:String="",offset:Int=0){lookupJob?.cancel();metadata=emptyList();metadataMore=false;lookupJob=viewModelScope.launch{try{val data=api.call("/channel-metadata/$field?q=${java.net.URLEncoder.encode(query,"UTF-8")}&limit=12&offset=$offset");metadata=data.getJSONArray("items").objects();metadataMore=data.optBoolean("more")}catch(e:Exception){if(e !is CancellationException)error=e.message}}}
    fun template(input:JSONObject,create:Boolean,onCreated:()->Unit={}){action{val value=api.call("/smart-channel-templates/${if(create)"instantiate" else "preview"}","POST",input);if(create){preview=null;loadChannels();refresh();onCreated()}else preview=value}}
    var editingChannel by mutableStateOf<JSONObject?>(null)
    var editingSchedule by mutableStateOf<JSONObject?>(null)
    fun editChannel(id:String){action{val c=api.call("/channels/$id");val s=api.call("/channels/$id/schedule/settings");editingSchedule=s;editingChannel=c}}
    fun saveChannel(c:JSONObject,s:JSONObject,onSaved:()->Unit){action{
        val id=c.getString("id");val body=JSONObject()
        for(key in listOf("number","name","shortName","description","groupId","enabled","visibility","sortOrder","logo","background","artworkMediaId","source")){if(c.has(key))body.put(key,c.get(key))}
        api.call("/channels/$id","PATCH",body)
        api.call("/channels/$id/schedule/settings","PATCH",s)
        api.call("/channels/$id/schedule/repair","POST")
        loadChannels();refresh();loadGuide();onSaved()
    }}
    fun schedule(id:String){action{api.call("/channels/$id/schedule/generate","POST");refresh()}}
    fun enabled(channel:JSONObject){action{val c=api.call("/channels/${channel.getString("id")}");val body=JSONObject();for(key in listOf("number","name","shortName","description","groupId","enabled","visibility","sortOrder","logo","background","artworkMediaId","source")){if(c.has(key))body.put(key,c.get(key))};body.put("enabled",!c.getBoolean("enabled"));api.call("/channels/${channel.getString("id")}","PATCH",body);loadChannels();refresh()}}
    fun createRecent(kind:String){action{val c=api.call("/channel-templates/recent","POST",JSONObject().put("kind",kind));api.call("/channels/${c.getString("id")}/schedule/generate","POST");refresh()}}
    private suspend fun window(path:String):List<Channel>{val result=mutableListOf<Channel>();var offset=0;do{val data=api.call(path+(if(path.contains('?'))"&" else "?")+"offset=$offset&limit=100");serverDelta=data.optLong("serverTime")-System.currentTimeMillis();timezone=data.optString("timezone","UTC");groups=data.optJSONArray("groups")?.objects()?.map{it.getString("id") to it.getString("name")}?:emptyList();val rows=data.getJSONArray("channels").objects().map(Channel::parse);result.addAll(rows);offset+=rows.size;if(rows.isEmpty())break}while(data.optBoolean("more"));return result}
    fun refresh(){if(!ready)return;action{channels=window("/guide/on-now");soon=window("/guide/starting-soon");ranked=api.call("/personalization/for-you").getJSONArray("items").objects().map{Ranked(Channel.parse(it.getJSONObject("channel")),Program.parse(it.getJSONObject("program")),it.getJSONArray("reasons").optString(0))};last=api.call("/users/me/last-channel").optString("channelId")}}
    fun loadGuide(start:Long=guideStart,group:String=guideGroup,favorites:Boolean=guideFavorites){guideGroup=group;guideFavorites=favorites;guideStart=start;guideJob?.cancel();guideJob=viewModelScope.launch{try{guide=window("/guide?start=${Instant.ofEpochMilli(start)}&end=${Instant.ofEpochMilli(start+3*3600000)}&group=$group&favorites=$favorites")}catch(e:Exception){if(e !is CancellationException)error=e.message}}}
    suspend fun details(id:String)=api.call("/guide/programs/$id")
    fun favorite(c:Channel){action{api.call("/users/me/favorites/${c.id}",if(c.favorite)"DELETE" else "POST");refresh();loadGuide()}}
    fun hide(c:Channel){action{api.call("/users/me/channels/${c.id}/hide","POST");refresh();loadGuide()}}
    fun number(number:String){action{val c=api.call("/watch/channel-number/$number");playback.tune(c.getString("id"))}}
    override fun onCleared(){linking?.cancel();playback.release();engine.stop()}
}
class MainActivity:ComponentActivity(){
    private val model:TvModel by viewModels()
    override fun onCreate(savedInstanceState:Bundle?){
        super.onCreate(savedInstanceState)
        val original=window.callback
        val consumedKeys=mutableSetOf<Int>()
        window.callback=object:android.view.Window.Callback by original {
            override fun dispatchKeyEvent(event:android.view.KeyEvent):Boolean {
                if(event.action==android.view.KeyEvent.ACTION_UP&&consumedKeys.remove(event.keyCode))return true
                if(model.remoteHandler?.invoke(event)==true){if(event.action==android.view.KeyEvent.ACTION_DOWN)consumedKeys.add(event.keyCode);return true}
                return original.dispatchKeyEvent(event)
            }
        }
        setContent{AfterglowApp(model)}
    }
    override fun onStop(){model.playback.stop();super.onStop()}
    override fun onResume(){super.onResume();if(model.ready)model.start()}
}
