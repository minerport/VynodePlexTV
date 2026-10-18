package tv.afterglow.client

import org.json.JSONArray
import org.json.JSONObject

fun JSONArray.objects():List<JSONObject> = (0 until length()).map{getJSONObject(it)}
data class Program(val id:String,val channelId:String,val title:String,val show:String,val start:Long,val end:Long,val art:String,val state:String,val restart:Boolean,val season:Int,val episode:Int) {
    val displayTitle get()=show.ifBlank{title}
    val context get()=if(show.isNotBlank())"S${season.toString().padStart(2,'0')} E${episode.toString().padStart(2,'0')} · $title" else ""
    companion object {fun parse(j:JSONObject)=Program(j.optString("id"),j.optString("channelId"),j.optString("title"),j.optString("showTitle"),j.optLong("start"),j.optLong("end"),j.optString("artwork"),j.optString("state"),j.optBoolean("canRestart",true),j.optInt("season"),j.optInt("episode"))}
}
data class Channel(val id:String,val number:String,val name:String,val logo:String,val favorite:Boolean,val recent:String,val programs:List<Program>) {
    companion object {fun parse(j:JSONObject)=Channel(j.getString("id"),j.optString("number"),j.optString("name"),j.optString("logo"),j.optBoolean("favorite"),j.optString("recentKind"),j.optJSONArray("programs")?.objects()?.map(Program::parse)?:emptyList())}
}
data class Ranked(val channel:Channel,val program:Program,val reason:String)
/** Stateless geometry and focus targeting are shared by rendering and tests. */
object Timeline {
    fun width(start:Long,end:Long,from:Long,to:Long,scale:Float):Float = ((minOf(end,to)-maxOf(start,from)).coerceAtLeast(0)/60000f)*scale
    fun nearest(programs:List<Program>,at:Long):Program?=programs.firstOrNull{at in it.start until it.end}?:programs.minByOrNull{ minOf(kotlin.math.abs(it.start-at),kotlin.math.abs(it.end-at))}
}

data class PlexLinkResult(val authorized:Boolean,val servers:List<JSONObject>){
    companion object {fun parse(value:JSONObject)=PlexLinkResult(value.optString("status")=="AUTHORIZED",value.optJSONArray("servers")?.objects()?:emptyList())}
}
