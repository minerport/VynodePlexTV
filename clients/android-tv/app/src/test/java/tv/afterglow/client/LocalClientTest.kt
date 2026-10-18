package tv.afterglow.client

import org.junit.Assert.*
import org.junit.Test
import org.json.JSONObject

class LocalClientTest {
 @Test fun pendingPlexApprovalDoesNotEndPolling(){
  val waiting=PlexLinkResult.parse(JSONObject("""{"status":"WAITING","servers":[]}"""))
  assertFalse(waiting.authorized)
  val ready=PlexLinkResult.parse(JSONObject("""{"status":"AUTHORIZED","servers":[{"id":"fixture"}]}"""))
  assertTrue(ready.authorized);assertEquals("fixture",ready.servers.single().getString("id"))
 }
 @Test fun authorizedEmptyAccountIsDistinctFromWaiting(){assertTrue(PlexLinkResult.parse(JSONObject("""{"status":"AUTHORIZED","servers":[]}""")).authorized)}
 @Test fun timelineUsesExactDurationsAndClipsWindow(){assertEquals(110f,Timeline.width(0,22*60000,0,180*60000,5f));assertEquals(25f,Timeline.width(-60000,5*60000,0,180*60000,5f));assertEquals(0f,Timeline.width(200*60000,210*60000,0,180*60000,5f))}
 @Test fun verticalNavigationPreservesTimeAndEndIsExclusive(){
  fun p(id:String,start:Long,end:Long)=Program(id,"ch",id,"",start,end,"","",true,0,0)
  val programs=listOf(p("a",0,100),p("b",100,200));assertEquals("b",Timeline.nearest(programs,150)?.id);assertEquals("b",Timeline.nearest(programs,100)?.id)
 }
 @Test fun episodeAndMovieMetadataRemainDistinct(){
  val ep=Program.parse(JSONObject("""{"id":"1","showTitle":"Fixture Show","title":"Episode","season":2,"episode":3}"""));assertEquals("Fixture Show",ep.displayTitle);assertEquals("S02 E03 · Episode",ep.context)
  val movie=Program.parse(JSONObject("""{"id":"2","title":"Fixture Movie"}"""));assertEquals("Fixture Movie",movie.displayTitle);assertEquals("",movie.context)
 }
}
