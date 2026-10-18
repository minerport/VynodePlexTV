package tv.afterglow.client

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.*
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import org.json.JSONTokener
import org.json.JSONArray
import java.net.URI
import java.util.concurrent.TimeUnit

class ApiException(val status:Int,message:String):Exception(message)
/** Local IPC adapter. No server address, pairing or external Afterglow account exists. */
class Api(private val engine:EmbeddedEngine){
    val server get()=engine.origin
    private val base=OkHttpClient.Builder().connectTimeout(5,TimeUnit.SECONDS).readTimeout(60,TimeUnit.SECONDS).followRedirects(false).followSslRedirects(false).build()
    val mediaClient=base.newBuilder().readTimeout(0,TimeUnit.SECONDS).addInterceptor{chain->
        val uri=URI(server);val request=chain.request()
        require(request.url.host=="127.0.0.1"&&request.url.port==uri.port)
        chain.proceed(request.newBuilder().header("Cookie","afterglow_session=${engine.token}").build())
    }.build()
    suspend fun call(path:String,method:String="GET",body:JSONObject?=null):JSONObject=withContext(Dispatchers.IO){
        require(path.startsWith("/")&&!path.startsWith("//"))
        val request=Request.Builder().url(server+"/api/v1"+path).header("Cookie","afterglow_session=${engine.token}").header("Origin",server).header("X-CSRF-Token",engine.csrf)
        if(method!="GET")request.method(method,(body?:JSONObject()).toString().toRequestBody("application/json".toMediaType()))
        base.newCall(request.build()).execute().use{r->val raw=JSONTokener(r.body?.string()?:"{}").nextValue();val data=if(raw is JSONArray)JSONObject().put("items",raw)else raw as JSONObject;if(!r.isSuccessful)throw ApiException(r.code,data.optString("error","Local operation failed"));data}
    }
}
