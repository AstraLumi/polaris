package net.astralumi.polaris

import android.annotation.SuppressLint
import android.app.AlertDialog
import android.app.DownloadManager
import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import android.graphics.Bitmap
import android.graphics.Typeface
import android.net.Uri
import android.os.Bundle
import android.os.Environment
import android.print.PrintManager
import android.text.InputType
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.view.ViewGroup.LayoutParams.MATCH_PARENT
import android.view.ViewGroup.LayoutParams.WRAP_CONTENT
import android.webkit.CookieManager
import android.webkit.URLUtil
import android.webkit.ValueCallback
import android.webkit.WebChromeClient
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.Button
import android.widget.EditText
import android.widget.FrameLayout
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.OnBackPressedCallback
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.view.ViewCompat
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.webkit.WebViewCompat
import androidx.webkit.WebViewFeature

/**
 * Polaris in a window of its own: the server's pages in a web view that keeps its sign-in,
 * with what a bare web view lacks (choosing files, downloads, printing, the back button,
 * a screen for when the server can't be reached).
 *
 * The pages are the server's, so the app never needs updating when Polaris does.
 */
class MainActivity : ComponentActivity() {
    private lateinit var web: WebView
    private lateinit var offline: View
    private lateinit var signedOut: View

    /** Where Polaris answers, without a trailing slash; empty until it is known. */
    private var server = ""
    private var failed = false
    private var chosenFiles: ValueCallback<Array<Uri>>? = null

    private val prefs by lazy { getSharedPreferences("polaris", Context.MODE_PRIVATE) }

    private val pickFiles = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
        val data = result.data
        val clip = data?.clipData
        val uris = when {
            result.resultCode != RESULT_OK -> null
            clip != null -> Array(clip.itemCount) { clip.getItemAt(it).uri }
            else -> WebChromeClient.FileChooserParams.parseResult(result.resultCode, data)
        }
        chosenFiles?.onReceiveValue(uris)
        chosenFiles = null
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        WindowCompat.setDecorFitsSystemWindows(window, false)

        web = WebView(this)
        offline = offlineScreen()
        signedOut = signedOutBar()
        val root = FrameLayout(this).apply {
            setBackgroundColor(getColor(R.color.background))
            addView(web, FrameLayout.LayoutParams(MATCH_PARENT, MATCH_PARENT))
            addView(offline, FrameLayout.LayoutParams(MATCH_PARENT, MATCH_PARENT))
            addView(signedOut, FrameLayout.LayoutParams(MATCH_PARENT, WRAP_CONTENT, Gravity.BOTTOM))
        }
        setContentView(root)

        // The page stays clear of the status bar, the gesture bar, a camera notch and the keyboard.
        ViewCompat.setOnApplyWindowInsetsListener(root) { view, insets ->
            val bars = insets.getInsets(
                WindowInsetsCompat.Type.systemBars() or WindowInsetsCompat.Type.displayCutout() or WindowInsetsCompat.Type.ime(),
            )
            view.setPadding(bars.left, bars.top, bars.right, bars.bottom)
            WindowInsetsCompat.CONSUMED
        }
        WindowCompat.getInsetsController(window, root).apply {
            isAppearanceLightStatusBars = false
            isAppearanceLightNavigationBars = false
        }

        setUpWeb()

        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                if (offline.visibility != View.VISIBLE && web.canGoBack()) web.goBack() else finish()
            }
        })

        server = cleanAddress(prefs.getString(KEY_SERVER, null) ?: BuildConfig.SERVER_URL) ?: ""
        if (server.isEmpty()) askForServer(first = true) else open()
    }

    override fun onPause() {
        super.onPause()
        // The sign-in cookie must survive the app being closed right now.
        CookieManager.getInstance().flush()
    }

    override fun onDestroy() {
        chosenFiles?.onReceiveValue(null)
        web.destroy()
        super.onDestroy()
    }

    private fun open() {
        // A new address: start over with a clean view, which then opens it.
        if (messagesFor.isNotEmpty() && messagesFor != server) return recreate()
        allowPageMessages()
        failed = false
        web.loadUrl("$server/")
    }

    // ---- the web view ---------------------------------------------------------------------

    @SuppressLint("SetJavaScriptEnabled")
    private fun setUpWeb() {
        web.setBackgroundColor(getColor(R.color.background))
        web.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true // the theme, drafts and the offline copy live there
            mediaPlaybackRequiresUserGesture = true
            setSupportMultipleWindows(false) // "open in a new tab" opens here; Back returns
            allowFileAccess = false
            allowContentAccess = false
        }
        CookieManager.getInstance().setAcceptCookie(true)

        web.webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
                if (!request.isForMainFrame || inApp(request.url)) return false
                // Only a link followed from one of Polaris's own pages leaves the app. Signing in
                // can pass through other addresses (an identity provider's, and back): all of
                // that journey, and anything the server redirects to, stays here, or the
                // sign-in would end up in the browser instead of in the app.
                val onPolaris = view.url?.let { isServer(Uri.parse(it)) } == true
                if (!onPolaris || request.isRedirect) return false
                openOutside(request.url)
                return true
            }

            override fun onPageStarted(view: WebView, url: String?, favicon: Bitmap?) {
                signedOut.visibility = View.GONE
            }

            override fun onPageFinished(view: WebView, url: String?) {
                if (!failed) offline.visibility = View.GONE
            }

            override fun onReceivedError(view: WebView, request: WebResourceRequest, error: WebResourceError) {
                if (!request.isForMainFrame) return
                failed = true
                offline.visibility = View.VISIBLE
            }

            // A call the page makes in the background being sent to the sign-in page means the
            // sign-in ran out while the app was open. Say so; reloading shows the sign-in page.
            override fun shouldInterceptRequest(view: WebView, request: WebResourceRequest): WebResourceResponse? {
                if (!request.isForMainFrame && isSignIn(request.url)) runOnUiThread { signedOut.visibility = View.VISIBLE }
                return null
            }
        }

        web.webChromeClient = object : WebChromeClient() {
            override fun onShowFileChooser(view: WebView, callback: ValueCallback<Array<Uri>>, params: FileChooserParams): Boolean {
                chosenFiles?.onReceiveValue(null)
                chosenFiles = callback
                return try {
                    pickFiles.launch(params.createIntent())
                    true
                } catch (e: ActivityNotFoundException) {
                    chosenFiles = null
                    false
                }
            }
        }

        // Exports and the story's .zip: Android's downloader fetches them with the page's sign-in.
        web.setDownloadListener { url, userAgent, disposition, mime, _ ->
            val uri = Uri.parse(url)
            if (!isServer(uri)) return@setDownloadListener openOutside(uri)
            val name = URLUtil.guessFileName(url, disposition, mime)
            val request = DownloadManager.Request(uri)
                .setTitle(name)
                .setMimeType(mime)
                .addRequestHeader("User-Agent", userAgent)
                .setNotificationVisibility(DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED)
                .setDestinationInExternalPublicDir(Environment.DIRECTORY_DOWNLOADS, name)
            CookieManager.getInstance().getCookie(url)?.let { request.addRequestHeader("Cookie", it) }
            getSystemService(DownloadManager::class.java).enqueue(request)
            Toast.makeText(this, getString(R.string.downloading, name), Toast.LENGTH_SHORT).show()
        }
    }

    /**
     * A web view has no window.print(). Give the server's pages (and only those: never the
     * sign-in page) one that asks the app, which prints the page as Android prints anything.
     */
    private var messagesFor = ""

    private fun allowPageMessages() {
        if (server.isEmpty() || messagesFor == server) return
        if (!WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER) ||
            !WebViewFeature.isFeatureSupported(WebViewFeature.DOCUMENT_START_SCRIPT)
        ) return
        messagesFor = server
        val pages = setOf(server)
        WebViewCompat.addWebMessageListener(web, "polarisApp", pages) { _, message, _, isMainFrame, _ ->
            if (isMainFrame && message.data == "print") {
                val title = web.title?.takeIf { it.isNotBlank() } ?: getString(R.string.app_name)
                getSystemService(PrintManager::class.java).print(title, web.createPrintDocumentAdapter(title), null)
            }
        }
        WebViewCompat.addDocumentStartJavaScript(web, "window.print = () => polarisApp.postMessage('print')", pages)
    }

    // ---- which address is what -------------------------------------------------------------

    private fun isServer(uri: Uri) = server.isNotEmpty() && originOf(uri) == server

    /** Cloudflare Access signs people in on the team's own cloudflareaccess.com address. */
    private fun isSignIn(uri: Uri) =
        uri.scheme == "https" && uri.host?.endsWith(".cloudflareaccess.com") == true

    private fun inApp(uri: Uri) = isServer(uri) || isSignIn(uri)

    private fun originOf(uri: Uri): String {
        val port = if (uri.port == -1 || uri.port == 443) "" else ":${uri.port}"
        return "${uri.scheme}://${uri.host?.lowercase()}$port"
    }

    /** "polaris.example.com/" → "https://polaris.example.com"; null when it is no https address. */
    private fun cleanAddress(typed: String): String? {
        var text = typed.trim()
        if (text.isEmpty()) return null
        if (!text.contains("://")) text = "https://$text"
        val uri = Uri.parse(text)
        if (uri.scheme?.lowercase() != "https" || uri.host.isNullOrEmpty()) return null
        return originOf(uri.buildUpon().scheme("https").build())
    }

    private fun openOutside(uri: Uri) {
        try {
            startActivity(Intent(Intent.ACTION_VIEW, uri))
        } catch (e: ActivityNotFoundException) {
            Toast.makeText(this, R.string.no_app_for_link, Toast.LENGTH_SHORT).show()
        }
    }

    // ---- the app's own few screens ---------------------------------------------------------

    private fun dp(value: Int) =
        TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_DIP, value.toFloat(), resources.displayMetrics).toInt()

    private fun label(text: Int, size: Float, color: Int, bold: Boolean = false) = TextView(this).apply {
        setText(text)
        textSize = size
        setTextColor(getColor(color))
        gravity = Gravity.CENTER
        if (bold) typeface = Typeface.DEFAULT_BOLD
    }

    private fun offlineScreen(): View = LinearLayout(this).apply {
        orientation = LinearLayout.VERTICAL
        gravity = Gravity.CENTER
        setBackgroundColor(getColor(R.color.background))
        setPadding(dp(32), dp(32), dp(32), dp(32))
        isClickable = true // nothing behind it is to be touched
        visibility = View.GONE
        addView(label(R.string.offline_title, 22f, R.color.text, bold = true))
        addView(label(R.string.offline_text, 15f, R.color.text_soft).apply { setPadding(0, dp(8), 0, dp(24)) })
        addView(Button(context).apply {
            setText(R.string.retry)
            setOnClickListener {
                failed = false
                if (web.url == null) open() else web.reload()
            }
        }, LinearLayout.LayoutParams(WRAP_CONTENT, WRAP_CONTENT))
        addView(Button(context, null, android.R.attr.borderlessButtonStyle).apply {
            setText(R.string.change_server)
            setTextColor(getColor(R.color.text_soft))
            setOnClickListener { askForServer(first = false) }
        }, LinearLayout.LayoutParams(WRAP_CONTENT, WRAP_CONTENT))
    }

    private fun signedOutBar(): View = LinearLayout(this).apply {
        orientation = LinearLayout.HORIZONTAL
        gravity = Gravity.CENTER_VERTICAL
        setBackgroundColor(getColor(R.color.icon_background))
        setPadding(dp(16), dp(4), dp(8), dp(4))
        visibility = View.GONE
        addView(
            label(R.string.signed_out, 14f, R.color.text).apply { gravity = Gravity.START },
            LinearLayout.LayoutParams(0, WRAP_CONTENT, 1f),
        )
        addView(Button(context, null, android.R.attr.borderlessButtonStyle).apply {
            setText(R.string.sign_in)
            setTextColor(getColor(R.color.accent))
            setOnClickListener { web.reload() } // the bar goes when the page starts loading
        })
    }

    private fun askForServer(first: Boolean) {
        val box = EditText(this).apply {
            setHint(R.string.server_hint)
            setText(server)
            inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_URI
            setSingleLine()
        }
        val holder = FrameLayout(this).apply {
            setPadding(dp(20), dp(8), dp(20), 0)
            addView(box)
        }
        val dialog = AlertDialog.Builder(this)
            .setTitle(R.string.server_title)
            .setView(holder)
            .setCancelable(!first)
            .setPositiveButton(R.string.save, null)
            .apply { if (!first) setNegativeButton(R.string.cancel, null) }
            .create()
        dialog.setOnShowListener {
            // Set here, not in the builder, so a wrong address keeps the dialog open.
            dialog.getButton(AlertDialog.BUTTON_POSITIVE).setOnClickListener {
                val address = cleanAddress(box.text.toString())
                if (address == null) {
                    box.error = getString(R.string.server_bad)
                    return@setOnClickListener
                }
                prefs.edit().putString(KEY_SERVER, address).apply()
                server = address
                dialog.dismiss()
                open()
            }
        }
        dialog.show()
    }

    private companion object {
        const val KEY_SERVER = "server_url"
    }
}
