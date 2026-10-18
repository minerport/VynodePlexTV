package tv.afterglow.client

import android.content.Context
import android.hardware.display.DisplayManager
import android.media.MediaCodecList
import android.view.Display
import org.json.JSONArray
import org.json.JSONObject

object Capabilities {
    fun detect(context:Context):JSONObject {
        val display=context.getSystemService(DisplayManager::class.java).getDisplay(Display.DEFAULT_DISPLAY)
        val height=display?.mode?.physicalHeight?:1080
        val video=mutableListOf<String>();val audio=mutableListOf<String>()
        val names=mapOf("video/avc" to "h264","video/hevc" to "hevc","video/x-vnd.on2.vp9" to "vp9","video/av01" to "av1","audio/mp4a-latm" to "aac","audio/ac3" to "ac3","audio/eac3" to "eac3","audio/vnd.dts" to "dca","audio/true-hd" to "truehd","audio/flac" to "flac","audio/opus" to "opus","audio/mpeg" to "mp3")
        var supportedHeight=480
        val codecHeights=mutableMapOf<String,Int>()
        var main10=false;var dolbyVision=false
        for(codec in MediaCodecList(MediaCodecList.REGULAR_CODECS).codecInfos.filter{!it.isEncoder}) for(mime in codec.supportedTypes) {
            if(mime=="video/dolby-vision")dolbyVision=true
            val name=names[mime]?:continue
            runCatching {val c=codec.getCapabilitiesForType(mime);if(mime.startsWith("video")) {
                if(mime=="video/hevc" && c.profileLevels.any{it.profile==android.media.MediaCodecInfo.CodecProfileLevel.HEVCProfileMain10 || it.profile==android.media.MediaCodecInfo.CodecProfileLevel.HEVCProfileMain10HDR10})main10=true
                val v=c.videoCapabilities
                val h=listOf(2160,1080,720,480).firstOrNull{it<=height && v.isSizeSupported(if(it==2160)3840 else if(it==1080)1920 else if(it==720)1280 else 854,it)}
                if(h!=null){video.add(name);supportedHeight=maxOf(supportedHeight,h);codecHeights[name]=maxOf(codecHeights[name]?:0,h)}
            } else audio.add(name)}
        }
        val hdr=display?.hdrCapabilities?.supportedHdrTypes?.toList()?.mapNotNull{when(it){Display.HdrCapabilities.HDR_TYPE_HDR10->"HDR10";Display.HdrCapabilities.HDR_TYPE_HLG->"HLG";Display.HdrCapabilities.HDR_TYPE_DOLBY_VISION->"DOLBY_VISION";Display.HdrCapabilities.HDR_TYPE_HDR10_PLUS->"HDR10_PLUS";else->null}}?:emptyList()
        val supportedHdr=hdr.filter{when(it){"DOLBY_VISION"->dolbyVision;"HDR10","HLG"->main10;else->false}}
        return JSONObject().put("videoMaxHeights",JSONObject(codecHeights as Map<*,*>)).put("clientType","android-tv").put("containers",JSONArray(listOf("mp4","mkv","webm","mpegts"))).put("videoCodecs",JSONArray(video.distinct())).put("audioCodecs",JSONArray(audio.distinct())).put("maxHeight",supportedHeight).put("hdrFormats",JSONArray(supportedHdr)).put("audioPassthrough",JSONArray()).put("subtitleFormats",JSONArray(listOf("srt","ass","ssa","webvtt","mov_text","pgs","dvd_subtitle"))).put("hls",true).put("quality","AUTO")
    }
}
