package tv.afterglow.client

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.*
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Text
import kotlinx.coroutines.delay
import org.json.JSONObject

/** Bounded choice pages keep remote navigation manageable; advanced settings stay editable. */
@Composable internal fun StudioChoices(options:List<Pair<String,String>>,selected:String,onSelect:(String)->Unit){
    options.chunked(3).forEach{row->Row(Modifier.fillMaxWidth(),horizontalArrangement=Arrangement.spacedBy(10.dp)){
        row.forEach{(id,label)->Action((if(id==selected)"✓ " else "")+label,Modifier.weight(1f)){onSelect(id)}}
        repeat(3-row.size){Spacer(Modifier.weight(1f))}
    }}
}
@Composable private fun StudioField(label:String,value:String,change:(String)->Unit){
    var open by remember{mutableStateOf(false)}
    Action("$label: ${value.ifBlank{"Automatic"}}",Modifier.fillMaxWidth()){open=true}
    if(open)DetailsModal({open=false}){focus->
        var draft by remember{mutableStateOf(value)}
        Column(Modifier.width(480.dp).background(LocalAppearance.current.surface).padding(24.dp),verticalArrangement=Arrangement.spacedBy(14.dp)){
            Text(label,fontSize=24.sp)
            Action("Cancel",Modifier.then(Modifier.focusRequesterCompat(focus))){open=false}
            BasicTextField(draft,{draft=it},singleLine=true,textStyle=TextStyle(color=Color.White,fontSize=22.sp),modifier=Modifier.fillMaxWidth().background(Color.Black.copy(alpha=.25f)).padding(16.dp))
            Action("Apply"){change(draft);open=false}
        }
    }
}
private fun Modifier.focusRequesterCompat(f:androidx.compose.ui.focus.FocusRequester)=this.then(Modifier.focusRequester(f))

@Composable internal fun ChannelStudio(model:TvModel){
    var creating by remember{mutableStateOf(false)}
    var category by remember{mutableStateOf("")}
    var query by remember{mutableStateOf("")}
    var offset by remember{mutableIntStateOf(0)}
    var draft by remember{mutableStateOf<JSONObject?>(null)}
    var name by remember{mutableStateOf("")}
    var number by remember{mutableStateOf("")}
    var mediaType by remember{mutableStateOf("")}
    var group by remember{mutableStateOf("auto")}
    var mode by remember{mutableStateOf("SMART_RANDOM")}
    var repeatDays by remember{mutableStateOf("30")}
    var yearFrom by remember{mutableStateOf("")}
    var yearTo by remember{mutableStateOf("")}
    var rating by remember{mutableStateOf("")}
    var advanced by remember{mutableStateOf(false)}
    var validation by remember{mutableStateOf("")}
    val scroll=rememberScrollState()
    fun options():JSONObject=JSONObject(draft.toString()).put("name",name).put("number",number).put("mediaType",mediaType).put("scheduleMode",mode).put("repeatDays",repeatDays.toIntOrNull()?:30).put("yearFrom",yearFrom.toIntOrNull()?:0).put("yearTo",yearTo.toIntOrNull()?:0).put("minimumRating",rating.toDoubleOrNull()?:0.0).also{if(group!="auto")it.put("groupId",group)}
    fun valid():Boolean{
        validation=when{
            name.isBlank()->"Enter a channel name."
            repeatDays.toIntOrNull() !in 0..365->"Repeat window must be 0–365 days."
            yearFrom.isNotBlank()&&yearFrom.toIntOrNull() !in 1880..2100->"Enter a valid starting year."
            yearTo.isNotBlank()&&yearTo.toIntOrNull() !in 1880..2100->"Enter a valid ending year."
            rating.isNotBlank()&&(rating.toDoubleOrNull()==null||rating.toDouble() !in 0.0..10.0)->"Rating must be 0–10."
            else->""
        };return validation.isEmpty()
    }
    fun choose(id:String,label:String){name=label;draft=JSONObject().put("template",category).put(if(category=="decade")"decade" else "selection",if(category=="decade")id.toInt() else id);mode=if(category=="comfort")"SEQUENTIAL" else "SMART_RANDOM";mediaType=if(category=="comfort")"episode" else "";model.preview=null;model.template(options(),false)}
    LaunchedEffect(Unit){model.loadChannels()}
    LaunchedEffect(category,query,offset){if(category.isNotEmpty()&&category!="decade"){delay(250);model.lookup(if(category=="comfort")"show" else category,query,offset)}}
    LaunchedEffect(creating,category,draft!=null,offset){scroll.scrollTo(0)}
    BackHandler(creating){if(draft!=null){draft=null;model.preview=null}else if(category.isNotEmpty())category="" else creating=false}
    if(model.editingChannel!=null&&model.editingSchedule!=null){ChannelConfiguration(model);return}
    Column(Modifier.fillMaxSize().verticalScroll(scroll),verticalArrangement=Arrangement.spacedBy(12.dp)){
        Text("Channel Studio",fontSize=30.sp)
        if(!creating){
            Text("Build a network around what you love. Refine it whenever you like.")
            Action("+ Create channel"){creating=true;category="";draft=null;advanced=false;group="auto";number="";yearFrom="";yearTo="";rating="";validation=""}
            model.configured.forEach{c->Row(Modifier.fillMaxWidth(),horizontalArrangement=Arrangement.spacedBy(12.dp)){
                Text("${c.optString("number")} · ${c.optString("name")}",Modifier.weight(1f),fontSize=21.sp,maxLines=2,overflow=TextOverflow.Ellipsis)
                Action("Edit"){model.editChannel(c.getString("id"))}
                Action(if(c.optBoolean("enabled"))"Disable" else "Enable"){model.enabled(c)}
            }}
        }else{
            Action("← ${if(draft!=null)"Choose different content" else if(category.isNotEmpty())"Channel types" else "Your channels"}"){if(draft!=null){draft=null;model.preview=null}else if(category.isNotEmpty())category="" else creating=false}
            if(category.isEmpty()){
                Text("1 · Choose a starting point",fontSize=24.sp)
                StudioChoices(listOf("genre" to "Genre","comfort" to "TV show","actor" to "Actor","director" to "Director","decade" to "Decade"),""){category=it;query="";offset=0;model.metadata=emptyList()}
                Row(horizontalArrangement=Arrangement.spacedBy(12.dp)){Action("Recently Added Movies"){if(!model.busy)model.createRecent("MOVIES")};Action("Recently Added TV"){if(!model.busy)model.createRecent("TV")}}
            }else if(draft==null){
                Text("2 · Choose ${if(category=="comfort")"a show" else "your $category"}",fontSize=24.sp)
                if(category=="decade")StudioChoices(listOf(1980,1990,2000,2010,2020).map{it.toString() to "${it}s"},""){choose(it,"${it}s")}
                else{
                    GuideSearch(query,"Search ${if(category=="comfort")"shows" else category}"){query=it;offset=0}
                    StudioChoices(model.metadata.map{it.getString("id") to "${it.getString("name")} · ${it.optInt("count")}"},""){id->choose(id,model.metadata.first{it.getString("id")==id}.getString("name"))}
                    if(model.metadata.isEmpty())Text("No choices loaded. Search another name or wait for the library lookup.")
                    Row(horizontalArrangement=Arrangement.spacedBy(12.dp)){if(offset>0)Action("← Previous 12"){offset-=12};Text("Page ${offset/12+1}");if(model.metadataMore)Action("Next 12 →"){offset+=12}}
                }
            }else{
                Text("3 · Make it yours",fontSize=24.sp)
                StudioField("Channel name",name){name=it}
                StudioField("Channel number",number){number=it}
                if(category!="comfort"){
                    Text("What should this channel include?")
                    StudioChoices(listOf("" to "Movies & TV","movie" to "Movies only","episode" to "Episodes only"),mediaType){mediaType=it;model.preview=null}
                }
                Text("Guide category")
                StudioChoices(listOf("auto" to "Automatic","" to "Ungrouped")+model.groups,group){group=it}
                Text("Episode / program order")
                StudioChoices(listOf("SEQUENTIAL" to if(category=="comfort")"Season & episode" else "Sequential","SMART_RANDOM" to "Smart random","RANDOM" to "Random"),mode){mode=it;model.preview=null}
                Action(if(advanced)"− Less detail" else "+ More customization"){advanced=!advanced}
                if(advanced){
                    if(mode=="SMART_RANDOM")StudioField("Repeat window (days)",repeatDays){repeatDays=it;model.preview=null}
                    if(category!="comfort"){
                        StudioField("Released from year",yearFrom){yearFrom=it;model.preview=null}
                        StudioField("Released through year",yearTo){yearTo=it;model.preview=null}
                        StudioField("Minimum rating (0–10)",rating){rating=it;model.preview=null}
                    }
                    Text("Artwork is selected automatically. Group and schedule settings remain editable after creation.")
                }
                if(validation.isNotBlank())Text(validation,color=Color(0xffffb4ab))
                model.preview?.let{Text("${it.optJSONObject("preview")?.optInt("count")} matching titles",color=LocalAppearance.current.accent)}
                Row(horizontalArrangement=Arrangement.spacedBy(12.dp)){
                    Action(if(model.busy)"Working…" else "Preview matches"){if(!model.busy&&valid())model.template(options(),false)}
                    Action("Create channel"){if(!model.busy&&valid())model.template(options(),true){creating=false;draft=null}}
                }
            }
        }
    }
}

@Composable private fun ChannelConfiguration(model:TvModel){
    val original=model.editingChannel!!;val settings=model.editingSchedule!!
    var name by remember(original){mutableStateOf(original.getString("name"))}
    var number by remember(original){mutableStateOf(original.getString("number"))}
    var group by remember(original){mutableStateOf(if(original.isNull("groupId"))"" else original.optString("groupId"))}
    var mode by remember(original){mutableStateOf(settings.getString("mode"))}
    var movieDays by remember(original){mutableStateOf(settings.optInt("movieRepeatDays").toString())}
    var episodeDays by remember(original){mutableStateOf(settings.optInt("episodeRepeatDays").toString())}
    var horizon by remember(original){mutableStateOf(settings.optInt("futureDays").toString())}
    var enabled by remember(original){mutableStateOf(original.optBoolean("enabled"))}
    var contentType by remember(original){mutableStateOf("keep")}
    var error by remember{mutableStateOf("")}
    fun close(){model.editingChannel=null;model.editingSchedule=null}
    BackHandler{close()}
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()),verticalArrangement=Arrangement.spacedBy(12.dp)){
        Action("← Your channels"){close()}
        Text("Customize channel",fontSize=30.sp)
        StudioField("Name",name){name=it};StudioField("Number",number){number=it}
        Text("Guide category · choose where this channel belongs")
        StudioChoices(listOf("" to "Automatic / ungrouped")+model.groups,group){group=it}
        if(original.getJSONObject("source").optString("type")=="RULE" && !original.getJSONObject("source").has("recent")){
            Text("Content type")
            StudioChoices(listOf("keep" to "Keep existing","movie" to "Movies only","episode" to "Episodes only"),contentType){contentType=it;if(group.isEmpty())group=if(it=="movie")"movies" else if(it=="episode")"tv" else ""}
        }
        Text("Programming order")
        StudioChoices(listOf("SEQUENTIAL" to "Sequential","SMART_RANDOM" to "Smart random","RANDOM" to "Random"),mode){mode=it}
        if(mode=="SMART_RANDOM"){
            StudioField("Movie repeat days",movieDays){movieDays=it}
            StudioField("Episode repeat days",episodeDays){episodeDays=it}
        }
        StudioField("Future guide days (1–30)",horizon){horizon=it}
        Action(if(enabled)"✓ Channel enabled" else "Channel disabled"){enabled=!enabled}
        Text("Saving repairs future programming. The current program and history stay intact.")
        if(error.isNotEmpty())Text(error,color=Color(0xffffb4ab))
        model.error?.let{Text(it,color=Color(0xffffb4ab))}
        Action(if(model.busy)"Saving…" else "Save changes"){
            if(!model.busy){
                if(name.isBlank()||number.isBlank()||movieDays.toIntOrNull() !in 0..365||episodeDays.toIntOrNull() !in 0..365||horizon.toIntOrNull() !in 1..30)error="Enter a name, number, valid repeat days and horizon."
                else{
                    val c=JSONObject(original.toString()).put("name",name).put("number",number).put("groupId",if(group.isEmpty())JSONObject.NULL else group).put("enabled",enabled)
                    if(contentType!="keep")c.put("source",restrictChannelMedia(original.getJSONObject("source"),contentType))
                    val s=JSONObject(settings.toString()).put("mode",mode).put("movieRepeatDays",movieDays.toInt()).put("episodeRepeatDays",episodeDays.toInt()).put("futureDays",horizon.toInt())
                    model.saveChannel(c,s){close()}
                }
            }
        }
    }
}

/** Apply an explicit type constraint without discarding nested/custom rule conditions. */
internal fun restrictChannelMedia(source:JSONObject,type:String):JSONObject{
    require(type=="movie"||type=="episode")
    val copy=JSONObject(source.toString())
    require(copy.optString("type")=="RULE"&&!copy.has("recent"))
    val rule=copy.getJSONObject("rule");val root=rule.getJSONObject("root")
    val children=org.json.JSONArray()
    if(root.optString("type")=="group"&&root.optString("operator")=="AND"){
        root.getJSONArray("children").objects().filterNot{it.optString("type")=="condition"&&it.optString("field")=="media_type"}.forEach{children.put(it)}
    }else children.put(root)
    children.put(JSONObject().put("type","condition").put("field","media_type").put("operator","is").put("value",type))
    rule.put("root",JSONObject().put("type","group").put("operator","AND").put("children",children))
    return copy
}
