package tv.afterglow.client

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.*
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.focus.FocusDirection
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.input.key.*
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.media3.ui.PlayerView
import androidx.tv.material3.*
import coil.ImageLoader
import coil.compose.AsyncImage
import coil.request.ImageRequest
import androidx.compose.ui.platform.LocalContext
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import org.json.JSONObject
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

private val Ink:Color @Composable get()=LocalAppearance.current.background
private val Accent:Color @Composable get()=LocalAppearance.current.accent
@Composable internal fun Action(text:String,modifier:Modifier=Modifier,onClick:()->Unit){
    var focused by remember{mutableStateOf(false)}
    val fill by androidx.compose.animation.animateColorAsState(if(focused)Accent else LocalAppearance.current.surface,animationSpec=androidx.compose.animation.core.tween(if(LocalMotion.current)160 else 0),label="focus")
    Box(modifier.onFocusChanged{focused=it.isFocused}.clip(RoundedCornerShape(7.dp))
        .background(fill)
        .border(if(focused)2.dp else 1.dp,if(focused)Color.White else Color(0xff354254),RoundedCornerShape(7.dp))
        .clickable(onClick=onClick).padding(horizontal=18.dp,vertical=12.dp)){
        Text(text,color=if(focused)Ink else Color(0xffedf2f7),fontSize=15.sp,fontWeight=FontWeight.Medium,maxLines=2)
    }
}
@Composable private fun NavItem(text:String,selected:Boolean,onClick:()->Unit){
    Column(horizontalAlignment=Alignment.CenterHorizontally,verticalArrangement=Arrangement.spacedBy(6.dp)){
        Action(text,onClick=onClick)
        Box(Modifier.width(24.dp).height(2.dp).background(if(selected)Accent else Color.Transparent))
    }
}
@Composable private fun SectionTitle(title:String){
    Row(Modifier.fillMaxWidth().padding(bottom=10.dp),verticalAlignment=Alignment.CenterVertically){
        Box(Modifier.width(3.dp).height(19.dp).background(Accent));Spacer(Modifier.width(10.dp))
        Text(title,fontSize=22.sp,fontWeight=FontWeight.SemiBold)
    }
}
@Composable internal fun Art(model:TvModel,path:String,modifier:Modifier=Modifier){
    val context=LocalContext.current
    val loader=remember(model){ImageLoader.Builder(context).okHttpClient(model.api.mediaClient).crossfade(model.motion).build()}
    if(path.isNotBlank())AsyncImage(model=ImageRequest.Builder(context).data(model.api.server+path).size(480,270).build(),imageLoader=loader,contentDescription=null,contentScale=androidx.compose.ui.layout.ContentScale.Crop,modifier=modifier.clip(RoundedCornerShape(6.dp)))
}
private fun clock(value:Long,zone:String)=DateTimeFormatter.ofPattern("h:mm a").withZone(ZoneId.of(zone)).format(Instant.ofEpochMilli(value))

@Composable fun AfterglowApp(model:TvModel){
    CompositionLocalProvider(LocalContentColor provides Color.White,LocalAppearance provides (appearanceThemes.firstOrNull{it.name==model.themeName}?:appearanceThemes.first()),LocalMotion provides model.motion,LocalCompact provides model.compact) { MaterialTheme(colorScheme=darkColorScheme(primary=Accent,background=Ink)){
        var page by rememberSaveable{mutableStateOf("Home")}
        var detail by remember{mutableStateOf<Program?>(null)}
        var detailData by remember{mutableStateOf<JSONObject?>(null)}
        var mini by remember{mutableStateOf(false)}
        var controls by remember{mutableStateOf(false)}
        var digits by remember{mutableStateOf("")}
        val viewing by model.playback.state.collectAsState()
        val watching=viewing.tune!=null||viewing.loading
        val scope=rememberCoroutineScope()
        val playerFocus=remember{FocusRequester()}
        LaunchedEffect(controls,watching){if(controls&&watching){delay(100);runCatching{playerFocus.requestFocus()}}}
        var options by remember{mutableStateOf<JSONObject?>(null)}
        LaunchedEffect(detail?.id){detailData=null;if(detail!=null)try{detailData=model.details(detail!!.id)}catch(e:Exception){model.error=e.message}}
        LaunchedEffect(digits){if(digits.isNotEmpty()){delay(1200);model.number(digits);digits=""}}
        LaunchedEffect(viewing.tune?.optString("id"),viewing.tune?.optJSONObject("program")?.optString("id")){if(watching){controls=true;delay(6000);if(options==null&&!mini)controls=false}}
        BackHandler {when{detail!=null->detail=null;options!=null->options=null;mini->mini=false;watching&&page=="Guide"->page="Player";watching->{model.playback.stop();page="Guide";model.loadGuide()};page!="Home"->page="Home";else->Unit}}
        fun surf(direction:Int){val id=viewing.pendingChannel?:viewing.tune?.optJSONObject("channel")?.optString("id");val i=model.channels.indexOfFirst{it.id==id};if(model.channels.isNotEmpty()){model.playback.tune(model.channels[Math.floorMod(i+direction,model.channels.size)].id);page="Player"}}
        val remote by rememberUpdatedState<(android.view.KeyEvent)->Boolean>({ event ->
            if(event.action!=android.view.KeyEvent.ACTION_DOWN||!watching||detail!=null||options!=null)false
            else when(val code=event.keyCode){
                in android.view.KeyEvent.KEYCODE_0..android.view.KeyEvent.KEYCODE_9->{digits=(digits+(code-android.view.KeyEvent.KEYCODE_0)).takeLast(5);true}
                android.view.KeyEvent.KEYCODE_CHANNEL_UP->{surf(1);true}
                android.view.KeyEvent.KEYCODE_CHANNEL_DOWN->{surf(-1);true}
                android.view.KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE->{model.playback.pause();controls=true;true}
                android.view.KeyEvent.KEYCODE_MEDIA_PLAY->{model.playback.player.play();controls=true;true}
                android.view.KeyEvent.KEYCODE_MEDIA_PAUSE->{model.playback.player.pause();controls=true;true}
                android.view.KeyEvent.KEYCODE_MEDIA_FAST_FORWARD->{model.playback.seek(30000);controls=true;true}
                android.view.KeyEvent.KEYCODE_MEDIA_REWIND->{model.playback.seek(-30000);controls=true;true}
                android.view.KeyEvent.KEYCODE_MENU,android.view.KeyEvent.KEYCODE_INFO->{if(page=="Player"){controls=true;true}else false}
                android.view.KeyEvent.KEYCODE_DPAD_UP,android.view.KeyEvent.KEYCODE_DPAD_DOWN,android.view.KeyEvent.KEYCODE_DPAD_LEFT,android.view.KeyEvent.KEYCODE_DPAD_RIGHT,android.view.KeyEvent.KEYCODE_DPAD_CENTER,android.view.KeyEvent.KEYCODE_ENTER->{if(page=="Player"&&!mini&&!controls){controls=true;true}else false}
                else->false
            }
        })
        DisposableEffect(model){model.remoteHandler={remote(it)};onDispose{model.remoteHandler=null}}
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(if(watching)listOf(Color.Black,Color.Black) else listOf(Color(0xff151923),Color(0xff090c12))))){
            if(watching){AndroidView(factory={ctx->PlayerView(ctx).apply{player=model.playback.player;useController=false;keepScreenOn=true;isFocusable=false;isFocusableInTouchMode=false;descendantFocusability=android.view.ViewGroup.FOCUS_BLOCK_DESCENDANTS}},modifier=Modifier.fillMaxSize())}
            if(!model.paired)ConnectScreen(model)
            else if(!watching||page!="Player")Column(Modifier.fillMaxSize().background(Ink.copy(alpha=if(watching)0.94f else 1f)).padding(horizontal=40.dp,vertical=24.dp)){
                Row(horizontalArrangement=Arrangement.spacedBy(14.dp),verticalAlignment=Alignment.CenterVertically){Text("AFTERGLOW",modifier=Modifier.padding(end=22.dp),fontSize=22.sp,color=Color.White,fontWeight=FontWeight.Bold,letterSpacing=3.sp);for(tab in listOf("Home","Guide","Channels","Settings"))NavItem(tab,page==tab){page=tab;if(tab=="Guide")model.loadGuide()};if(watching)Action("Return to TV"){page="Player"}}
                Spacer(Modifier.height(20.dp))
                if(model.error!=null)Row{Text(model.error!!,modifier=Modifier.weight(1f));Action("Retry"){model.refresh()}}
                when(page){"Guide"->NativeGuide(model){detail=it};"Settings"->TvSettings(model);"Channels"->ChannelStudio(model);else->HomeScreen(model,{c->model.playback.tune(c.id);page="Player"},{detail=it})}
            }
            if(watching&&page=="Player"){
                val t=viewing.tune;val channel=t?.optJSONObject("channel");val program=t?.optJSONObject("program");val branding=t?.optJSONObject("branding")
                if(!controls&&!mini&&branding?.optBoolean("bug")==true){val corner=branding.optString("corner");val alignment=when(corner){"top-left"->Alignment.TopStart;"bottom-left"->Alignment.BottomStart;"bottom-right"->Alignment.BottomEnd;else->Alignment.TopEnd};Art(model,if(branding.optString("compactLogoId").isNotEmpty())"/api/v1/watch/channels/${channel?.optString("id")}/artwork/compact" else channel?.optString("logo")?:"",Modifier.align(alignment).padding(40.dp).size(branding.optInt("size",72).dp).alpha(branding.optDouble("opacity",0.35).toFloat()))}
                val nextUp=t!=null&&t.optString("mode")=="LIVE"&&t.optString("origin")=="LIVE"&&model.playback.player.isPlaying&&(program?.optLong("end")?:0)-t.optLong("serverTime") in 1..((branding?.optInt("nextUpSeconds",45)?:45)*1000L)
                if(nextUp&&!controls&&!mini)Column(Modifier.align(Alignment.BottomEnd).padding(40.dp).background(Ink.copy(alpha=.9f),RoundedCornerShape(12.dp)).padding(20.dp)){Text("COMING UP NEXT",color=Accent);Text(t?.optJSONObject("next")?.optString("showTitle")?.ifBlank{t.optJSONObject("next")?.optString("title")}?:"Programming resumes shortly",fontSize=26.sp)}
                if(viewing.loading)Text("Tuning ${model.channels.firstOrNull{it.id==viewing.pendingChannel}?.name?:"channel"}…",fontSize=30.sp,modifier=Modifier.align(Alignment.Center).background(Ink).padding(24.dp))
                if(viewing.error!=null)Column(Modifier.align(Alignment.Center).background(Ink).padding(24.dp)){Text(viewing.error!!);Action("Retry playback"){model.playback.retry()};Action("Back to guide"){model.playback.stop();page="Guide";model.loadGuide()}}
                if(t?.optString("delivery")=="METADATA_PRESENTATION")Column(Modifier.align(Alignment.Center).background(Ink).padding(36.dp)){Text("COMING UP NEXT",color=Accent);Text(t.optJSONObject("next")?.optString("title")?:"Station break",fontSize=36.sp)}
                if(controls)Column(Modifier.align(Alignment.BottomStart).fillMaxWidth().background(Brush.verticalGradient(listOf(Color.Transparent,Ink))).padding(40.dp)){
                    Text("${channel?.optString("number")?:""}  ${channel?.optString("name")?:""}",color=Accent,fontSize=22.sp)
                    Text(program?.optString("showTitle")?.ifBlank{program.optString("title")}?:"",fontSize=30.sp,fontWeight=FontWeight.Bold)
                    Text("${program?.optString("title")?:""} · ${t?.optString("mode")?:""} · Next: ${t?.optJSONObject("next")?.optString("title")?:"—"}")
                    LazyRow(horizontalArrangement=Arrangement.spacedBy(12.dp)){item{Action("Play / Pause",Modifier.focusRequester(playerFocus)){model.playback.pause()}};item{Action("Go Live"){model.playback.goLive()}};item{Action("From beginning"){if(channel!=null){model.playback.tune(channel.getString("id"),"FROM_BEGINNING",program?.optString("id"))}}};item{Action("Mini-guide"){mini=true;controls=false}};item{Action("Guide"){page="Guide";model.loadGuide()}};item{Action("Audio · Subtitles · Quality"){scope.launch{try{options=model.playback.options()}catch(e:Exception){model.error=e.message}}}};item{Action("CH +"){surf(1)}};item{Action("CH −"){surf(-1)}};item{Action("Hide controls"){controls=false}}}
                }
                if(mini)Column(Modifier.align(Alignment.CenterEnd).width(430.dp).fillMaxHeight().background(Ink.copy(alpha=.94f)).padding(28.dp)){
                    Text("ON NOW / NEXT",color=Accent,fontSize=22.sp);Action("Close mini-guide"){mini=false}
                    LazyColumn(verticalArrangement=Arrangement.spacedBy(12.dp)){items(model.channels,key={it.id}){c->Action("${c.number} · ${c.name}\n${c.programs.firstOrNull()?.displayTitle?:"Station break"}",Modifier.fillMaxWidth()){model.playback.tune(c.id);mini=false}}}
                }
                if(digits.isNotEmpty())Text(digits,fontSize=48.sp,color=Accent,modifier=Modifier.align(Alignment.TopEnd).padding(40.dp))
            }
            if(detail!=null){val p=detail!!;DetailsModal(onClose={detail=null}) { firstFocus -> Column(Modifier.fillMaxSize().background(Ink.copy(alpha=.97f)).padding(48.dp).verticalScroll(rememberScrollState())){
                Row{Art(model,detailData?.optString("background")?:p.art,Modifier.width(300.dp).height(170.dp));Column(Modifier.padding(24.dp)){Text(p.displayTitle,fontSize=34.sp);Text(p.context);Text("${clock(p.start,model.timezone)} – ${clock(p.end,model.timezone)}")}}
                Text(detailData?.optString("summary")?:"Loading details…",maxLines=6,modifier=Modifier.padding(vertical=20.dp))
                Row(horizontalArrangement=Arrangement.spacedBy(12.dp)){Action("Back",Modifier.focusRequester(firstFocus)){detail=null};if(p.start<=model.now()&&p.end>model.now())Action("Watch Live"){model.playback.tune(p.channelId);detail=null;page="Player"};if(p.start<=model.now()&&p.restart)Action("Watch From Beginning"){model.playback.tune(p.channelId,"FROM_BEGINNING",p.id);detail=null;page="Player"};model.channels.firstOrNull{it.id==p.channelId}?.let{c->Action(if(c.favorite)"Unfavorite" else "Favorite"){model.favorite(c)}}}
            }}}
            if(options!=null)PlaybackOptions(options!!,model,{options=null})
        }
    }
}

}
@Composable private fun ConnectScreen(model:TvModel){
    Column(Modifier.fillMaxSize().padding(48.dp).verticalScroll(rememberScrollState()),verticalArrangement=Arrangement.spacedBy(18.dp)){
        Text("AFTERGLOW TV",fontSize=38.sp,color=Accent)
        Text("Your television. Your library. All on this device.",fontSize=24.sp)
        Text("Connect Plex to build your channels. No Afterglow server or Docker installation required.")
        if(!model.ready){Text(if(model.error==null)"Preparing your local programming engine…" else "Unable to start");Action("Retry"){model.start()}}
        else if(model.pairing==null)Action(if(model.busy)"Connecting…" else "Sign in with Plex"){if(!model.busy)model.signInPlex()}
        else if(model.plexChoices.isEmpty()){
            val url=model.pairing!!.optString("authUrl")
            Text("Scan with your phone and approve Plex sign-in.",fontSize=24.sp)
            val bitmap=remember(url){val matrix=com.google.zxing.MultiFormatWriter().encode(url,com.google.zxing.BarcodeFormat.QR_CODE,440,440);android.graphics.Bitmap.createBitmap(440,440,android.graphics.Bitmap.Config.ARGB_8888).apply{for(y in 0 until 440)for(x in 0 until 440)setPixel(x,y,if(matrix[x,y])android.graphics.Color.BLACK else android.graphics.Color.WHITE)}}
            Image(bitmap.asImageBitmap(),"Plex sign-in QR code",Modifier.size(260.dp))
            Action("Start sign-in again"){model.signInPlex()}
        }else{
            Text("Choose your Plex media server",fontSize=26.sp)
            model.plexChoices.forEach{s->Text(s.optString("name"),fontSize=22.sp);Action("Connect automatically (secure)"){if(!model.busy)model.connectPlex(s,"")};s.optJSONArray("connections")?.objects()?.forEach{c->Action(c.optString("uri")){if(!model.busy)model.connectPlex(s,c.getString("uri"))}}}
        }
        model.error?.let{Text(it,color=Color(0xffffb4ab))}
    }
}

@Composable private fun HomeScreen(model:TvModel,tune:(Channel)->Unit,details:(Program)->Unit){
    LazyColumn(verticalArrangement=Arrangement.spacedBy(22.dp)){
        item{
            val hero=model.channels.firstOrNull{it.id==model.last}?:model.ranked.firstOrNull()?.channel?:model.channels.firstOrNull()
            if(hero!=null)Box(Modifier.fillMaxWidth().height(if(LocalCompact.current)225.dp else 250.dp).clip(RoundedCornerShape(12.dp)).background(LocalAppearance.current.surface)){
                Art(model,hero.programs.firstOrNull()?.art?:hero.logo,Modifier.align(Alignment.CenterEnd).fillMaxHeight().fillMaxWidth(.65f))
                Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(listOf(Ink,Ink.copy(alpha=.85f),Color.Transparent))))
                Column(Modifier.padding(28.dp).width(480.dp),verticalArrangement=Arrangement.spacedBy(10.dp)){
                    Text(if(hero.id==model.last)"BACK TO YOUR CHANNEL" else "ON AIR NOW",color=Accent,fontSize=11.sp,letterSpacing=2.sp,fontWeight=FontWeight.Bold)
                    Text(hero.programs.firstOrNull()?.displayTitle?:hero.name,fontSize=32.sp,fontWeight=FontWeight.Bold,maxLines=2,overflow=TextOverflow.Ellipsis)
                    Text("${hero.number}  /  ${hero.name}",color=Color(0xffa9b7c8),fontSize=16.sp)
                    hero.programs.firstOrNull()?.let{p->Text("${clock(p.start,model.timezone)} – ${clock(p.end,model.timezone)}",color=Color(0xffa9b7c8),fontSize=13.sp)}
                    Action("Watch Live"){tune(hero)}
                }
            }
        }
        if(model.ranked.isNotEmpty())item{SectionTitle("For You Right Now");LazyRow(horizontalArrangement=Arrangement.spacedBy(16.dp)){items(model.ranked,key={it.program.id}){r->Card(onClick={details(r.program)},modifier=Modifier.width(if(LocalCompact.current)210.dp else 260.dp)){Column(Modifier.background(LocalAppearance.current.surface).padding(10.dp)){Art(model,r.program.art,Modifier.fillMaxWidth().height(125.dp));Text(r.program.displayTitle,maxLines=1,overflow=TextOverflow.Ellipsis);Text(r.reason,color=Accent,fontSize=15.sp)}}}}}
        val rows=listOf("Favorites" to model.channels.filter{it.favorite},"On Now" to model.channels,"Recently Added" to model.channels.filter{it.recent.isNotEmpty()})
        rows.forEach{(name,channels)->if(channels.isNotEmpty())item{SectionTitle(name);LazyRow(horizontalArrangement=Arrangement.spacedBy(16.dp)){items(channels,key={it.id}){c->Card(onClick={tune(c)},modifier=Modifier.width(if(LocalCompact.current)210.dp else 260.dp)){Column(Modifier.background(LocalAppearance.current.surface).padding(10.dp)){Art(model,c.programs.firstOrNull()?.art?:c.logo,Modifier.fillMaxWidth().height(125.dp));Text("${c.number} · ${c.name}");Text(c.programs.firstOrNull()?.displayTitle?:"Station break",maxLines=1)}}}}}}
        if(model.soon.isNotEmpty())item{SectionTitle("Starting Soon");LazyRow(horizontalArrangement=Arrangement.spacedBy(16.dp)){items(model.soon,key={it.id}){c->c.programs.firstOrNull()?.let{p->Action("${p.displayTitle}\n${clock(p.start,model.timezone)}"){details(p)}}}}}
        if(model.channels.isEmpty())item{Text(if(model.busy)"Loading your television…" else "Choose Settings to select Plex libraries, synchronize and create your channels.")}
    }
}
@Composable private fun TvSettings(model:TvModel){
    LaunchedEffect(Unit){while(true){model.setupRefresh();delay(5000)}}
    Column(Modifier.verticalScroll(rememberScrollState()),verticalArrangement=Arrangement.spacedBy(14.dp)){
        SectionTitle("Appearance")
        Text("Choose your atmosphere. Saved on this device.",color=Color(0xffa9b7c8))
        LazyRow(horizontalArrangement=Arrangement.spacedBy(10.dp)){items(appearanceThemes){theme->
            Action((if(model.themeName==theme.name)"✓ " else "")+theme.name){model.appearance(theme=theme.name)}
        }}
        Row(horizontalArrangement=Arrangement.spacedBy(12.dp)){
            Action(if(model.compact)"Layout: Compact" else "Layout: Cinematic"){model.appearance(dense=!model.compact)}
            Action(if(model.motion)"Effects: Smooth focus" else "Effects: Reduced motion"){model.appearance(animate=!model.motion)}
        }
        SectionTitle("Settings · On this device")
        Text("App ${BuildConfig.VERSION_NAME} · ${android.os.Build.MODEL}\nChannels, schedules and history are stored on this device.")
        SectionTitle("Your libraries")
        Text("Automatic checks every ${model.syncMinutes} minutes while Afterglow is running. Matching channels update after a successful sync.",color=Color(0xffa9b7c8))
        model.servers.forEach{Text("${it.optString("name")} · ${it.optString("status")}")}
        model.libraries.forEach{l->Action("${if(l.optBoolean("selected"))"✓" else "○"} ${l.optString("title")} · ${l.optString("type")} · ${l.optInt("itemCount")} items"){if(l.optBoolean("supported"))model.selectLibrary(l)}}
        Row(horizontalArrangement=Arrangement.spacedBy(12.dp)){Action("Refresh"){model.setupRefresh()};Action("Sync selected libraries"){model.sync()}}
        model.jobs.take(6).forEach{j->Text("${j.optString("type")} · ${j.optString("status")} · ${j.optInt("processed")} / ${if(j.isNull("total"))"?" else j.optInt("total")} items\n${j.optString("error")}")}
        Text("Create your first channels",fontSize=24.sp)
        Text("After synchronization, these channels use the last 30 days of Plex additions.")
        Row(horizontalArrangement=Arrangement.spacedBy(12.dp)){Action("Recently Added Movies"){model.createRecent("MOVIES")};Action("Recently Added TV"){model.createRecent("TV")}}
        Text("Home stops playback. Return and choose Continue Watching to tune live again.")
    }
}

@Composable private fun PlaybackOptions(options:JSONObject,model:TvModel,close:()->Unit){DetailsModal(onClose=close){ first -> Column(Modifier.fillMaxSize().background(Ink.copy(alpha=.98f)).padding(40.dp).verticalScroll(rememberScrollState()),verticalArrangement=Arrangement.spacedBy(14.dp)){
    SectionTitle("Picture & sound");Action("Back",Modifier.focusRequester(first),onClick=close)
    Text("Quality");Row(horizontalArrangement=Arrangement.spacedBy(10.dp)){for(q in listOf("AUTO","ORIGINAL","4K","1080p","720p","480p"))Action(q){model.playback.option("quality",q);close()}}
    Text("Version");Action("Auto version"){model.playback.option("versionId","");close()}
    val versions=options.optJSONArray("versions")?.objects()?:emptyList()
    versions.forEach{v->Action("${v.optString("resolution")} · ${v.optString("videoCodec")} · ${v.optString("audioCodec")} ${v.optString("hdr")}"){model.playback.option("versionId",v.getString("mediaId"));close()}}
    val selected=versions.firstOrNull{it.optString("mediaId")==model.playback.capabilities.optString("versionId")}?:versions.firstOrNull()
    Text("Audio");selected?.optJSONArray("streams")?.objects()?.filter{it.optInt("type")==2}?.forEach{s->Action("${s.optString("language")} · ${s.optString("title")} · ${s.optString("codec")} ${s.optInt("channels")} channels"){model.playback.option("audioStreamId",s.getString("id"));close()}}
    Text("Subtitles");Action("Off"){model.playback.option("subtitleStreamId","off");close()};selected?.optJSONArray("streams")?.objects()?.filter{it.optInt("type")==3}?.forEach{s->Action("${s.optString("language")} · ${s.optString("title")} ${if(s.optBoolean("forced"))"Forced" else ""} ${if(s.optBoolean("hearingImpaired"))"SDH" else ""}"){model.playback.option("subtitleStreamId",s.getString("id"));close()}}
}}}

/** A separate modal focus window prevents D-pad events reaching the underlying guide. */
@Composable internal fun DetailsModal(onClose:()->Unit,content:@Composable (FocusRequester)->Unit){
    val first=remember{FocusRequester()}
    Dialog(onDismissRequest=onClose,properties=DialogProperties(usePlatformDefaultWidth=false)){
        LaunchedEffect(Unit){first.requestFocus()}
        content(first)
    }
}
