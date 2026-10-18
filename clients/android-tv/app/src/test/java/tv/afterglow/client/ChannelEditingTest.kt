package tv.afterglow.client
import org.json.JSONObject
import org.junit.Assert.*
import org.junit.Test
class ChannelEditingTest {
 @Test fun contentTypeReplacesTopLevelConstraintWithoutLosingGenre(){
 val source=JSONObject("""{"type":"RULE","rule":{"version":1,"root":{"type":"group","operator":"AND","children":[{"type":"condition","field":"genre","operator":"is","value":"genre"},{"type":"condition","field":"media_type","operator":"is","value":"episode"}]}}}""")
 val changed=restrictChannelMedia(source,"movie").getJSONObject("rule").getJSONObject("root").getJSONArray("children")
 assertEquals(2,changed.length());assertEquals("genre",changed.getJSONObject(0).getString("field"));assertEquals("movie",changed.getJSONObject(1).getString("value"))
 assertEquals("episode",source.getJSONObject("rule").getJSONObject("root").getJSONArray("children").getJSONObject(1).getString("value"))
 }
 @Test fun preservesOrGroup(){
 val source=JSONObject("""{"type":"RULE","rule":{"version":1,"root":{"type":"group","operator":"OR","children":[]}}}""")
 val result=restrictChannelMedia(source,"episode").getJSONObject("rule").getJSONObject("root")
 assertEquals("AND",result.getString("operator"));assertEquals("OR",result.getJSONArray("children").getJSONObject(0).getString("operator"))
 }
}
