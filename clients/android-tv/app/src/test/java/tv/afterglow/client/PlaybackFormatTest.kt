package tv.afterglow.client
import org.junit.Assert.assertEquals
import org.junit.Test
class PlaybackFormatTest {
 @Test fun plexHLSSelectsPlaylistParserWithoutFileExtension(){
  for(mime in listOf("application/vnd.apple.mpegurl","application/x-mpegURL","application/vnd.apple.mpegurl; charset=utf-8")) assertEquals("application/x-mpegURL",PlaybackFormat.mime(mime,"PLEX_HLS"))
 }
 @Test fun directMediaRetainsItsParser(){assertEquals("video/mp4",PlaybackFormat.mime("video/mp4","DIRECT_PLAY"));assertEquals("video/x-matroska",PlaybackFormat.mime("video/x-matroska","DIRECT_PLAY"))}
}
