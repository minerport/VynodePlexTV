package tv.afterglow.client

/** Proxy routes have no extension; normalize Plex's MIME alias for Media3. */
object PlaybackFormat {
 fun mime(raw:String,delivery:String):String {
  val value=raw.substringBefore(';').trim().lowercase()
  return if(delivery=="PLEX_HLS" || value=="application/vnd.apple.mpegurl" || value=="application/x-mpegurl" || value=="audio/mpegurl" || value=="audio/x-mpegurl") "application/x-mpegURL" else value
 }
}
