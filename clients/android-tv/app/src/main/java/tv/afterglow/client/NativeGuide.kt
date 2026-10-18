package tv.afterglow.client

import androidx.compose.foundation.*
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clipToBounds
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.key.*
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Text
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

private val GuideBackground:Color @Composable get()=LocalAppearance.current.background
private val GuideSurface:Color @Composable get()=LocalAppearance.current.surface
private val GuideBorder=Color(0xff3d4b5d)
private val GuideAccent:Color @Composable get()=LocalAppearance.current.accent
private val GuideMuted=Color(0xffa9b5c5)
private fun timeLabel(ms:Long,zone:String)=DateTimeFormatter.ofPattern("h:mm a").withZone(ZoneId.of(zone)).format(Instant.ofEpochMilli(ms))

@Composable private fun GuideAction(label:String,selected:Boolean=false,modifier:Modifier=Modifier,onClick:()->Unit){
 var focused by remember{mutableStateOf(false)}
 Box(modifier.heightIn(min=34.dp).onFocusChanged{focused=it.isFocused}
  .background(if(focused)GuideAccent else if(selected)Color(0xff354157) else GuideSurface,RoundedCornerShape(4.dp))
  .border(if(focused)2.dp else 1.dp,if(focused)GuideAccent else GuideBorder,RoundedCornerShape(4.dp))
  .clickable(role=Role.Button,onClick=onClick).padding(horizontal=10.dp,vertical=7.dp)){
  Text(label,color=if(focused)Color(0xff100d1d) else Color.White,fontSize=13.sp,maxLines=1,overflow=TextOverflow.Ellipsis)
 }
}

/** Compact, bordered television guide; data always comes from the local broadcast timeline. */
@Composable internal fun NativeGuide(model:TvModel,details:(Program)->Unit){
 var group by model::guideGroup;var favorites by model::guideFavorites
 var query by remember{mutableStateOf("")};var tick by remember{mutableLongStateOf(model.now())}
 var focused by remember{mutableStateOf<Program?>(null)};var summary by remember{mutableStateOf("")}
 var anchor by remember{mutableLongStateOf(model.now())}
 val vertical=rememberLazyListState();val scope=rememberCoroutineScope()
 val from=model.guideStart;val to=from+90*60000L
 val channels=model.guide.filter{query.isBlank()||it.name.contains(query,true)||it.number.contains(query)}
 val requesters=remember(channels){channels.flatMap{it.programs}.associate{it.id to FocusRequester()}}
 val selected=focused?.takeIf{p->channels.any{c->c.programs.any{it.id==p.id}}}?:channels.flatMap{it.programs}.firstOrNull{tick in it.start until it.end}
 LaunchedEffect(group,favorites){model.loadGuide(model.guideStart,group,favorites)}
 LaunchedEffect(Unit){while(true){delay(1000);tick=model.now()}}
 LaunchedEffect(selected?.id){summary="";if(selected!=null){delay(200);try{summary=model.details(selected.id).optString("summary")}catch(_:Exception){summary="Details temporarily unavailable"}}}
 LaunchedEffect(model.guide){val p=channels.flatMap{it.programs}.firstOrNull{it.id==model.selectedProgram&&it.end>from&&it.start<to};if(p!=null){withFrameNanos{};runCatching{requesters[p.id]?.requestFocus()}}}
 fun shift(ms:Long){model.loadGuide(from+ms,group,favorites)}
 fun day(delta:Long){model.loadGuide(Instant.ofEpochMilli(from).atZone(ZoneId.of(model.timezone)).plusDays(delta).toInstant().toEpochMilli(),group,favorites)}
 Row(Modifier.fillMaxSize(),horizontalArrangement=Arrangement.spacedBy(12.dp)){
  Column(Modifier.width(142.dp).fillMaxHeight().background(GuideBackground,RoundedCornerShape(6.dp)).border(1.dp,GuideBorder,RoundedCornerShape(6.dp)).padding(8.dp),verticalArrangement=Arrangement.spacedBy(6.dp)){
   Text("YOUR COLLECTION",color=GuideMuted,fontSize=10.sp,letterSpacing=1.sp,modifier=Modifier.padding(vertical=5.dp))
   GuideSearch(query){query=it}
   GuideAction("★ Favorites",favorites,Modifier.fillMaxWidth()){favorites=!favorites}
   GuideAction("All channels",!favorites&&group.isEmpty(),Modifier.fillMaxWidth()){group="";favorites=false}
   Column(Modifier.weight(1f).verticalScroll(rememberScrollState()),verticalArrangement=Arrangement.spacedBy(5.dp)){
    model.groups.forEach{g->GuideAction(g.second,group==g.first,Modifier.fillMaxWidth()){group=g.first;favorites=false}}
   }
   Text("↑↓ Channels\n←→ Programs\nOK Details",fontSize=11.sp,lineHeight=18.sp,color=GuideMuted)
  }
  Column(Modifier.weight(1f).fillMaxHeight()){
   Row(Modifier.fillMaxWidth().height(132.dp).background(GuideBackground,RoundedCornerShape(6.dp)).padding(10.dp),horizontalArrangement=Arrangement.spacedBy(16.dp)){
    Art(model,selected?.art?:"",Modifier.width(186.dp).fillMaxHeight().clipToBounds())
    Column(Modifier.weight(1f),verticalArrangement=Arrangement.spacedBy(4.dp)){
     Text(channels.firstOrNull{it.id==selected?.channelId}?.let{"${it.number}  ·  ${it.name}"}?:"AFTERGLOW GUIDE",color=GuideAccent,fontSize=12.sp)
     Text(selected?.displayTitle?:"Your television, at a glance",fontSize=23.sp,fontWeight=FontWeight.SemiBold,maxLines=1,overflow=TextOverflow.Ellipsis)
     Text(selected?.let{listOf(it.context,"${timeLabel(it.start,model.timezone)} – ${timeLabel(it.end,model.timezone)}").filter{v->v.isNotBlank()}.joinToString(" · ")}?:"Browse channels and choose a program",color=GuideMuted,fontSize=12.sp,maxLines=1,overflow=TextOverflow.Ellipsis)
     Text(summary,fontSize=12.sp,lineHeight=17.sp,maxLines=3,overflow=TextOverflow.Ellipsis,color=Color(0xffd4dce6))
    }
   }
   Row(Modifier.fillMaxWidth().padding(vertical=8.dp),horizontalArrangement=Arrangement.spacedBy(6.dp),verticalAlignment=Alignment.CenterVertically){
    GuideAction("‹ Day"){day(-1)};GuideAction("‹ 30m"){shift(-30*60000)};GuideAction("NOW",tick in from until to){model.loadGuide(model.now()-5*60000,group,favorites)};GuideAction("30m ›"){shift(30*60000)};GuideAction("Day ›"){day(1)}
    Text(DateTimeFormatter.ofPattern("EEE, MMM d").withZone(ZoneId.of(model.timezone)).format(Instant.ofEpochMilli(from)),fontSize=12.sp,color=GuideMuted,modifier=Modifier.padding(start=6.dp))
   }
   BoxWithConstraints(Modifier.weight(1f).fillMaxWidth()){
    val channelWidth=126.dp;val timelineWidth=(maxWidth-channelWidth).coerceAtLeast(100.dp)
    Column{
     Row(Modifier.fillMaxWidth().height(37.dp).background(GuideBackground),verticalAlignment=Alignment.CenterVertically){
      Text("CHANNEL",Modifier.width(channelWidth).padding(start=8.dp),fontSize=10.sp,color=GuideMuted)
      Box(Modifier.width(timelineWidth).fillMaxHeight().clipToBounds()){
       repeat(3){i->Text(timeLabel(from+i*30*60000L,model.timezone),Modifier.offset(x=timelineWidth*(i/3f)).padding(start=6.dp),fontSize=12.sp,color=GuideMuted)}
       if(tick in from until to)Text("NOW",Modifier.padding(top=19.dp).offset(x=timelineWidth*((tick-from).toFloat()/(to-from))).background(GuideAccent).padding(horizontal=3.dp),fontSize=10.sp,color=Color.Black)
      }
     }
     LazyColumn(state=vertical,verticalArrangement=Arrangement.spacedBy(3.dp),modifier=Modifier.weight(1f)){
      itemsIndexed(channels,key={_,c->c.id}){row,c->
       val programs=c.programs.filter{it.end>from&&it.start<to}
       Row(Modifier.fillMaxWidth().height(54.dp).clipToBounds()){
        Row(Modifier.width(channelWidth).fillMaxHeight().background(GuideBackground).padding(6.dp),verticalAlignment=Alignment.CenterVertically,horizontalArrangement=Arrangement.spacedBy(6.dp)){
         Art(model,c.logo,Modifier.size(30.dp))
         Column{Text(c.number+(if(c.favorite)" ★" else ""),color=GuideAccent,fontSize=12.sp);Text(c.name,fontSize=12.sp,maxLines=2,lineHeight=15.sp,overflow=TextOverflow.Ellipsis)}
        }
        Box(Modifier.width(timelineWidth).fillMaxHeight().clipToBounds()){
         if(programs.isEmpty())Box(Modifier.fillMaxSize().background(GuideBackground).border(1.dp,GuideBorder).padding(8.dp)){Text("No programming in this window",color=GuideMuted,fontSize=12.sp)}
         programs.forEachIndexed{index,p->
          val left=(maxOf(p.start,from)-from).toFloat()/(to-from);val right=(minOf(p.end,to)-from).toFloat()/(to-from)
          if(right>left){var cellFocus by remember(p.id){mutableStateOf(false)}
           val width=timelineWidth*(right-left);val live=tick in p.start until p.end
           Box(Modifier.offset(x=timelineWidth*left).width(width).fillMaxHeight().padding(end=2.dp).clipToBounds()
            .focusRequester(requesters.getValue(p.id)).onFocusChanged{cellFocus=it.isFocused;if(it.isFocused){focused=p;model.selectedProgram=p.id;model.selectedChannel=c.id;anchor=anchor.coerceIn(p.start,p.end-1)}}
            .onPreviewKeyEvent{event->if(event.type!=KeyEventType.KeyDown)false else when(event.key){
             Key.DirectionUp,Key.DirectionDown->{val targetRow=row+if(event.key==Key.DirectionUp)-1 else 1;val adjacent=channels.getOrNull(targetRow);val target=adjacent?.let{Timeline.nearest(it.programs.filter{v->v.end>from&&v.start<to},anchor)};if(target==null)false else{scope.launch{if(vertical.layoutInfo.visibleItemsInfo.none{it.index==targetRow})vertical.animateScrollToItem(targetRow);withFrameNanos{};requesters[target.id]?.requestFocus()};true}}
             Key.DirectionLeft->{if(index==0){shift(-30*60000);true}else false}
             Key.DirectionRight->{if(index==programs.lastIndex){shift(30*60000);true}else false}
             else->false}}
            .background(if(cellFocus)GuideAccent else if(live)Color(0xff303d50) else GuideSurface)
            .border(if(cellFocus)2.dp else 1.dp,if(cellFocus)Color.White else GuideBorder)
            .clickable(role=Role.Button){details(p)}.padding(horizontal=8.dp,vertical=6.dp)){
            Column{
             Text(p.displayTitle,fontSize=14.sp,fontWeight=if(cellFocus||live)FontWeight.SemiBold else FontWeight.Normal,color=if(cellFocus)Color(0xff100d1d) else Color.White,maxLines=if(width>150.dp)1 else 2,lineHeight=17.sp,overflow=TextOverflow.Ellipsis)
             if(width>150.dp)Text(p.context.ifBlank{"${timeLabel(p.start,model.timezone)} · ${(p.end-p.start)/60000} min"},fontSize=10.sp,color=if(cellFocus)Color(0xff282036) else GuideMuted,maxLines=1,overflow=TextOverflow.Ellipsis)
            }
           }
          }
         }
         if(tick in from until to)Box(Modifier.offset(x=timelineWidth*((tick-from).toFloat()/(to-from))).width(2.dp).fillMaxHeight().background(GuideAccent))
        }
       }
      }
      if(channels.isEmpty())item{Text("No channels in this view. Choose All channels or clear the search.",Modifier.padding(20.dp),color=GuideMuted,fontSize=14.sp)}
     }
     Text("${channels.size} channels  ·  ${timeLabel(from,model.timezone)} – ${timeLabel(to,model.timezone)}",fontSize=11.sp,color=GuideMuted,modifier=Modifier.padding(top=6.dp))
    }
   }
  }
 }
}

/** Navigation focuses only the button; text input exists only after explicit activation. */
@Composable internal fun GuideSearch(query:String,title:String="Search channels",onQuery:(String)->Unit){
 var open by remember{mutableStateOf(false)}
 var draft by remember{mutableStateOf(query)}
 var restore by remember{mutableStateOf(false)}
 val button=remember{FocusRequester()}
 val keyboard=androidx.compose.ui.platform.LocalSoftwareKeyboardController.current
 LaunchedEffect(open,restore){if(!open&&restore){button.requestFocus();restore=false}}
 fun close(){keyboard?.hide();open=false;restore=true}
 GuideAction("⌕ Search",query.isNotBlank(),Modifier.fillMaxWidth().focusRequester(button)){draft=query;open=true}
 if(query.isNotBlank()){
  Text(query,color=GuideMuted,fontSize=12.sp,maxLines=1,overflow=TextOverflow.Ellipsis)
  GuideAction("Clear search",modifier=Modifier.fillMaxWidth()){onQuery("")}
 }
 if(open){
  val input=remember{FocusRequester()}
  androidx.compose.ui.window.Dialog(onDismissRequest={close()}){
   LaunchedEffect(Unit){input.requestFocus();keyboard?.show()}
   Column(Modifier.width(440.dp).background(GuideBackground,RoundedCornerShape(10.dp)).border(1.dp,GuideBorder,RoundedCornerShape(10.dp)).padding(24.dp),verticalArrangement=Arrangement.spacedBy(18.dp)){
      Text(title,fontSize=24.sp,color=Color.White)
    BasicTextField(draft,{draft=it},singleLine=true,textStyle=TextStyle(color=Color.White,fontSize=20.sp),
     keyboardOptions=androidx.compose.foundation.text.KeyboardOptions(imeAction=androidx.compose.ui.text.input.ImeAction.Search),
     keyboardActions=androidx.compose.foundation.text.KeyboardActions(onSearch={onQuery(draft);close()}),
     modifier=Modifier.fillMaxWidth().focusRequester(input).background(GuideSurface).border(1.dp,GuideAccent).padding(12.dp))
    Row(horizontalArrangement=Arrangement.spacedBy(12.dp)){
     GuideAction("Apply search"){onQuery(draft);close()}
     GuideAction("Cancel"){close()}
    }
   }
  }
 }
}
