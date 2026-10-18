package tv.afterglow.client

import android.content.Context
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.io.File
import java.security.SecureRandom
import java.util.TimeZone

/** The entire programming engine ships in the APK and owns only this installation's private data. */
class EmbeddedEngine(private val context:Context,private val store:SecureStore) {
    var origin=""; private set
    val token=random();val csrf=random()
    private var process:Process?=null
    private fun random():String=ByteArray(32).also{SecureRandom().nextBytes(it)}.joinToString(""){"%02x".format(it)}
    suspend fun start()=withContext(Dispatchers.IO){
        if(process?.isAlive==true)return@withContext
        val protected=store.read()
        if(!protected.has("engineKey")){protected.put("engineKey",random());store.save(protected)}
        val data=File(context.filesDir,"programming").apply{mkdirs()}
        val binary=File(context.applicationInfo.nativeLibraryDir,"libafterglow.so")
        check(binary.exists()){ "This APK does not contain an engine for this device architecture" }
        // Android has no writable /tmp. SQLite spills large imports/sorts to disk.
        // Supply its directory before process launch (native libc snapshots env).
        val temporary=File(context.cacheDir,"engine-tmp").apply{mkdirs()}
        check(temporary.isDirectory&&temporary.canWrite()){ "Local temporary storage is unavailable" }
        val builder=ProcessBuilder(binary.absolutePath).redirectError(ProcessBuilder.Redirect.PIPE)
        builder.environment()["TMPDIR"]=temporary.absolutePath
        builder.environment()["SQLITE_TMPDIR"]=temporary.absolutePath
        val child=builder.start();process=child
        val connectivity=context.getSystemService(android.net.ConnectivityManager::class.java)
        val dns=connectivity.getLinkProperties(connectivity.activeNetwork)?.dnsServers?.mapNotNull{it.hostAddress}?:emptyList()
        val boot=JSONObject().put("DNS",org.json.JSONArray(dns)).put("Dir",data.absolutePath).put("Token",token).put("CSRF",csrf).put("Key",protected.getString("engineKey")).put("ClientID",store.installationId).put("Timezone",TimeZone.getDefault().id)
        child.outputStream.write((boot.toString()+"\n").toByteArray());child.outputStream.flush()
        // Only a public loopback address is emitted; credentials never enter logs or arguments.
        val reader=child.inputStream.bufferedReader()
        val deadline=android.os.SystemClock.elapsedRealtime()+30000
        while(!reader.ready()&&child.isAlive&&android.os.SystemClock.elapsedRealtime()<deadline)Thread.sleep(50)
        if(!reader.ready()){child.destroy();process=null;error("Local programming engine could not start. Your data is preserved.")}
        val line=reader.readLine()?:error("Local engine stopped during startup")
        origin=JSONObject(line).getString("origin")
        require(origin.startsWith("http://127.0.0.1:"))
    }
    fun stop(){runCatching{process?.outputStream?.close()};process=null}
}
