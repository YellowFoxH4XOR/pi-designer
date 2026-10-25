// pidesigner-render — offscreen WKWebView screenshot + page diagnostics.
//
// Usage:
//   pidesigner-render --url <file:// or http:// URL> --width 1440 --height 900 \
//                     --out /abs/path.png [--full-page] [--timeout 15]
//
// Renders one page in an offscreen WKWebView at an exact CSS viewport
// (device scale 1), waits for load + fonts + images + a short settle, then
// writes a PNG and prints exactly one JSON line to stdout.
//
//   {"ok":true,"png":...,"width":...,"height":...,"scrollWidth":...,
//    "scrollHeight":...,"overflowX":false,"errors":[...],
//    "brokenImages":[...],"title":"..."}
//
// Exit code 0 on success, 1 on any failure (including the hard timeout).
//
// The process never shows a window and never activates: the app runs with the
// .prohibited activation policy and the render window is parked far offscreen.

import AppKit
import CoreGraphics
import WebKit

// MARK: - Constants

private let settleDelay: TimeInterval = 0.4
private let pdSeekWait: TimeInterval = 3.0
private let fontImageTimeout: TimeInterval = 2.0
private let maxFullPageHeight = 6000
/// Full pages are captured the way a visitor sees them: scrolled one viewport
/// at a time at the frame height, so 100vh sections keep their size and
/// scroll-triggered reveals have fired.
private let revealStepDelay: TimeInterval = 0.12
private let revealSettleDelay: TimeInterval = 1.2
private let tileDelay: TimeInterval = 0.15

// MARK: - Options

private struct RenderOptions {
    var urlText = ""
    var url: URL?
    var width = 1440
    var height = 900
    var outPath = ""
    var fullPage = false
    var slide = 0 // 1-based slide to bring into view; 0 = none
    var at: TimeInterval = 0 // animation time to seek to before capture; 0 = as loaded
    var wait: TimeInterval = 0 // real time to let the page run before capture (loaders, intros)
    var timeout: TimeInterval = 15
}

private func usageText() -> String {
    """
    usage: pidesigner-render --url <file:// or http:// URL> --width 1440 --height 900 \
    --out /abs/path.png [--full-page] [--slide N] [--at MS] [--wait MS] [--timeout 15]
    """
}

// MARK: - Output

/// Writes exactly one JSON line and never anything else to stdout.
private func emitJSON(_ object: [String: Any]) {
    let data: Data
    do {
        data = try JSONSerialization.data(withJSONObject: object, options: [])
    } catch {
        data = Data("{\"ok\":false,\"error\":\"could not encode JSON\"}".utf8)
    }
    var line = data
    line.append(0x0a)
    FileHandle.standardOutput.write(line)
}

private func fail(_ message: String) -> Never {
    emitJSON(["ok": false, "error": message])
    exit(1)
}

// MARK: - Argument parsing

private func absolutePath(_ path: String) -> String {
    let expanded = (path as NSString).expandingTildeInPath
    if expanded.hasPrefix("/") { return expanded }
    return FileManager.default.currentDirectoryPath + "/" + expanded
}

private func parseOptions() -> RenderOptions {
    var options = RenderOptions()
    var iterator = CommandLine.arguments.dropFirst().makeIterator()

    func nextValue(_ flag: String) -> String {
        guard let value = iterator.next() else { fail("\(flag) requires a value") }
        return value
    }

    while let arg = iterator.next() {
        switch arg {
        case "--url":
            options.urlText = nextValue(arg)
        case "--width":
            let raw = nextValue(arg)
            guard let value = Int(raw), value > 0, value <= 10000 else {
                fail("invalid --width: \(raw)")
            }
            options.width = value
        case "--height":
            let raw = nextValue(arg)
            guard let value = Int(raw), value > 0, value <= 10000 else {
                fail("invalid --height: \(raw)")
            }
            options.height = value
        case "--out":
            options.outPath = nextValue(arg)
        case "--timeout":
            let raw = nextValue(arg)
            guard let value = Double(raw), value > 0 else {
                fail("invalid --timeout: \(raw)")
            }
            options.timeout = value
        case "--full-page":
            options.fullPage = true
        case "--slide":
            let raw = nextValue(arg)
            guard let value = Int(raw), value > 0 else {
                fail("invalid --slide: \(raw)")
            }
            options.slide = value
        case "--at":
            // Capture a moment of an animation: its clocks are seeked to this time.
            let raw = nextValue(arg)
            guard let value = Int(raw), value >= 0, value <= 60000 else {
                fail("invalid --at: \(raw) (0–60000 ms)")
            }
            options.at = TimeInterval(value) / 1000
        case "--wait":
            // Let a live page run (preloaders, intro sequences) before capturing.
            let raw = nextValue(arg)
            guard let value = Int(raw), value >= 0, value <= 20000 else {
                fail("invalid --wait: \(raw) (0–20000 ms)")
            }
            options.wait = TimeInterval(value) / 1000
        case "-h", "--help":
            FileHandle.standardError.write(Data((usageText() + "\n").utf8))
            exit(0)
        default:
            fail("unknown argument: \(arg)")
        }
    }

    // A long wait still leaves the normal budget for loading and capture.
    options.timeout += options.wait
    guard !options.urlText.isEmpty else { fail("missing --url") }
    guard let url = URL(string: options.urlText) else {
        fail("invalid --url: \(options.urlText)")
    }
    guard let scheme = url.scheme?.lowercased() else {
        fail("invalid --url (missing scheme): \(options.urlText)")
    }
    guard scheme == "file" || scheme == "http" || scheme == "https" else {
        fail("unsupported URL scheme: \(scheme)")
    }
    if scheme == "file" {
        guard FileManager.default.fileExists(atPath: url.path) else {
            fail("file not found: \(url.path)")
        }
    }
    options.url = url

    guard !options.outPath.isEmpty else { fail("missing --out") }
    options.outPath = absolutePath(options.outPath)

    return options
}

// MARK: - Injected scripts

private enum Scripts {
    /// Injected at document start so page errors from the very first script
    /// are captured. Stored on a non-enumerable window property.
    static let errorCapture = #"""
    (function () {
      if (window.__pdErrors) return;
      var errors = [];
      try {
        Object.defineProperty(window, '__pdErrors', {
          value: errors, writable: false, configurable: false, enumerable: false
        });
      } catch (e) { window.__pdErrors = errors; }

      function push(message) {
        if (!message) return;
        var text = String(message);
        if (errors.indexOf(text) === -1 && errors.length < 200) errors.push(text);
      }
      function describe(value) {
        try {
          if (value instanceof Error) {
            return String(value.name || 'Error') + ': ' + String(value.message || '');
          }
          if (typeof value === 'string') return value;
          if (value === undefined || value === null) return String(value);
          return JSON.stringify(value);
        } catch (e) {
          return String(value);
        }
      }
      window.addEventListener('error', function (event) {
        if (event && event.message) {
          var where = event.filename
            ? ' (at ' + event.filename + ':' + event.lineno + ':' + event.colno + ')'
            : '';
          push(String(event.message) + where);
        }
      }, false);
      window.addEventListener('unhandledrejection', function (event) {
        push('Unhandled rejection: ' + describe(event ? event.reason : undefined));
      }, false);
      var originalConsoleError = console.error;
      console.error = function () {
        var parts = [];
        for (var i = 0; i < arguments.length; i++) parts.push(describe(arguments[i]));
        push('console.error: ' + parts.join(' '));
        return originalConsoleError.apply(console, arguments);
      };
    })();
    """#

    /// Waits for fonts, images, and a bounded fallback.
    static let readyBody = """
    var fonts = (document.fonts && document.fonts.ready)
      ? document.fonts.ready.catch(function () { return null; })
      : Promise.resolve(null);
    var images = Array.prototype.slice.call(document.images).map(function (img) {
      if (img.complete) return null;
      return new Promise(function (resolve) {
        var done = function () { resolve(null); };
        img.addEventListener('load', done, { once: true });
        img.addEventListener('error', done, { once: true });
      });
    });
    var capped = new Promise(function (resolve) {
      setTimeout(resolve, \(Int(fontImageTimeout * 1000)));
    });
    await Promise.race([Promise.all([fonts].concat(images)), capped]);
    return true;
    """

    /// Measures the document and collects diagnostics.
    static let measure = #"""
    (function () {
      var doc = document.documentElement;
      var body = document.body || doc;
      var scrollWidth = Math.max(doc.scrollWidth || 0, body.scrollWidth || 0, doc.clientWidth || 0);
      var scrollHeight = Math.max(doc.scrollHeight || 0, body.scrollHeight || 0, doc.clientHeight || 0);
      // Fixed-position content never grows the scroll box, yet a fixed box
      // that starts inside the viewport and runs past its right edge is
      // visibly clipped. Fold only those boxes into scrollWidth. In-flow
      // content already counts through scrollWidth; content inside its own
      // scroll container and off-canvas drawers parked beyond the right edge
      // are intentional and must not be reported as page overflow.
      var viewW = window.innerWidth, viewH = window.innerHeight;
      var rightEdge = 0;
      var nodes = body ? body.getElementsByTagName('*') : [];
      for (var k = 0; k < nodes.length; k++) {
        var el = nodes[k];
        if (!el.getClientRects || el.getClientRects().length === 0) continue;
        var style = getComputedStyle(el);
        if (style.position !== 'fixed' || style.visibility === 'hidden' || style.opacity === '0') continue;
        var box = el.getBoundingClientRect();
        if (box.width <= 0 || box.left >= viewW || box.bottom <= 0 || box.top >= viewH) continue;
        if (box.right > rightEdge) rightEdge = box.right;
      }
      if (rightEdge > scrollWidth) scrollWidth = Math.ceil(rightEdge);
      var errors = [];
      if (Array.isArray(window.__pdErrors)) {
        for (var i = 0; i < window.__pdErrors.length; i++) {
          errors.push(String(window.__pdErrors[i]));
        }
      }
      var brokenImages = [];
      var images = Array.prototype.slice.call(document.images);
      for (var j = 0; j < images.length; j++) {
        var img = images[j];
        if (img.complete && img.naturalWidth === 0) {
          var src = img.currentSrc || img.src || img.getAttribute('src') || '';
          brokenImages.push(String(src));
        }
      }
      return {
        scrollWidth: scrollWidth,
        scrollHeight: scrollHeight,
        errors: errors,
        brokenImages: brokenImages,
        title: document.title || ''
      };
    })()
    """#
}

// MARK: - Renderer

private final class Renderer: NSObject, WKNavigationDelegate, WKUIDelegate {
    private let options: RenderOptions
    private let debugEnabled = ProcessInfo.processInfo.environment["PD_RENDER_DEBUG"] == "1"

    private var window: NSWindow!
    private var webView: WKWebView!
    private var finished = false
    /// Animations moved to --at; nil when no time was requested.
    private var seeked: Int?
    private var timeoutWork: DispatchWorkItem?

    init(options: RenderOptions) {
        self.options = options
        super.init()
    }

    // MARK: Lifecycle

    func start() {
        let width = CGFloat(options.width)
        let height = CGFloat(options.height)
        let frame = NSRect(x: 0, y: 0, width: width, height: height)

        // Offscreen, non-activating host window. Parked far away so nothing
        // is ever drawn over the user's desktop.
        let window = NSWindow(
            contentRect: frame,
            styleMask: [.borderless],
            backing: .buffered,
            defer: false
        )
        window.isReleasedWhenClosed = false
        window.backgroundColor = .white
        window.isOpaque = true
        window.hasShadow = false
        window.ignoresMouseEvents = true
        window.animationBehavior = .none
        window.level = .normal
        window.collectionBehavior = [.canJoinAllSpaces, .stationary, .ignoresCycle]
        window.setFrameOrigin(NSPoint(x: -30000, y: -30000))
        self.window = window

        let configuration = WKWebViewConfiguration()
        configuration.websiteDataStore = .nonPersistent()
        configuration.defaultWebpagePreferences.allowsContentJavaScript = true
        // A file:// document otherwise gets an opaque origin: WebKit strips
        // uncaught error details down to "Script error." Relaxed only for
        // this local preview renderer so the model sees real messages.
        configuration.preferences.setValue(true, forKey: "allowFileAccessFromFileURLs")
        configuration.userContentController.addUserScript(
            WKUserScript(
                source: Scripts.errorCapture,
                injectionTime: .atDocumentStart,
                forMainFrameOnly: false
            )
        )

        let webView = WKWebView(frame: frame, configuration: configuration)
        webView.autoresizingMask = [.width, .height]
        webView.navigationDelegate = self
        webView.uiDelegate = self
        // The parked window counts as occluded, so WebKit would suspend
        // requestAnimationFrame after one frame: canvas, WebGL and GSAP pages
        // would never advance past their first frame or a preloader. Opt out
        // where WebKit offers the switch.
        let occlusion = NSSelectorFromString("_setWindowOcclusionDetectionEnabled:")
        if webView.responds(to: occlusion) {
            webView.perform(occlusion, with: false)
        }
        self.webView = webView

        window.contentView = webView
        window.orderFrontRegardless()

        // Scrolling a full page tile by tile needs time beyond one capture.
        let timeout = options.timeout + (options.fullPage ? 10 : 0)
        let timeoutWork = DispatchWorkItem { [weak self] in
            self?.finishFailure("render timed out after \(Int(timeout))s")
        }
        self.timeoutWork = timeoutWork
        DispatchQueue.main.asyncAfter(deadline: .now() + timeout, execute: timeoutWork)

        logWindowState("before load")

        if options.url?.isFileURL == true, let fileURL = options.url {
            // Allow the page to read sibling assets from its own directory.
            webView.loadFileURL(fileURL, allowingReadAccessTo: fileURL.deletingLastPathComponent())
        } else if let url = options.url {
            var request = URLRequest(url: url)
            request.timeoutInterval = timeout
            webView.load(request)
        } else {
            finishFailure("missing URL")
        }
    }

    private func finishFailure(_ message: String) {
        guard !finished else { return }
        finished = true
        timeoutWork?.cancel()
        emitJSON(["ok": false, "error": message])
        exit(1)
    }

    private func finishSuccess(width: Int, height: Int, scrollWidth: Double, scrollHeight: Double,
                               errors: [String], brokenImages: [String], title: String) {
        guard !finished else { return }
        finished = true
        timeoutWork?.cancel()
        emitJSON([
            "ok": true,
            "png": options.outPath,
            "width": width,
            "height": height,
            "scrollWidth": Int(scrollWidth.rounded()),
            "scrollHeight": Int(scrollHeight.rounded()),
            "overflowX": scrollWidth > Double(width) + 1,
            "errors": Array(errors.prefix(50)),
            "brokenImages": Array(brokenImages.prefix(50)),
            "title": title,
            "seeked": seeked.map { $0 as Any } ?? NSNull()
        ])
        exit(0)
    }

    // MARK: Navigation

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        waitForReady()
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        finishFailure("navigation failed: \(error.localizedDescription)")
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!,
                 withError error: Error) {
        finishFailure("navigation failed: \(error.localizedDescription)")
    }

    func webViewWebContentProcessDidTerminate(_ webView: WKWebView) {
        finishFailure("web content process terminated")
    }

    // JS dialogs must never block the render: dismiss them immediately.
    func webView(_ webView: WKWebView, runJavaScriptAlertPanelWithMessage message: String,
                 initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping () -> Void) {
        completionHandler()
    }

    func webView(_ webView: WKWebView, runJavaScriptConfirmPanelWithMessage message: String,
                 initiatedByFrame frame: WKFrameInfo,
                 completionHandler: @escaping (Bool) -> Void) {
        completionHandler(false)
    }

    func webView(_ webView: WKWebView, runJavaScriptTextInputPanelWithPrompt prompt: String,
                 defaultText: String?, initiatedByFrame frame: WKFrameInfo,
                 completionHandler: @escaping (String?) -> Void) {
        completionHandler(defaultText)
    }

    private func waitForReady() {
        webView.callAsyncJavaScript(
            Scripts.readyBody,
            arguments: [:],
            in: nil,
            in: .page
        ) { [weak self] result in
            guard let self = self, !self.finished else { return }
            switch result {
            case .failure(let error):
                self.finishFailure("readiness check failed: \(error.localizedDescription)")
            case .success:
                self.showSlideIfRequested { [weak self] in
                    DispatchQueue.main.asyncAfter(deadline: .now() + (self?.options.wait ?? 0)) { [weak self] in
                    self?.seekIfRequested { [weak self] in
                        DispatchQueue.main.asyncAfter(deadline: .now() + settleDelay) { [weak self] in
                            self?.measure()
                        }
                    }
                    }
                }
            }
        }
    }

    /// Brings the requested slide of a deck into view: slides are elements
    /// labelled with data-screen-label or classed .slide, in document order.
    /// Scroll-snap decks scroll to it; class-toggling decks get it as .active.
    private func showSlideIfRequested(then next: @escaping () -> Void) {
        guard options.slide > 0 else { next(); return }
        let script = """
        (function (n) {
          var list = Array.prototype.slice.call(document.querySelectorAll('[data-screen-label], .slide'));
          if (n > list.length) return list.length;
          var el = list[n - 1];
          if (el.classList.contains('slide')) {
            list.forEach(function (s) { if (s.classList.contains('slide')) s.classList.toggle('active', s === el); });
          }
          el.scrollIntoView({ behavior: 'instant', block: 'start', inline: 'start' });
          return -1;
        })(\(options.slide))
        """
        webView.evaluateJavaScript(script) { [weak self] value, error in
            guard let self = self, !self.finished else { return }
            if let error = error {
                self.finishFailure("could not show slide \(self.options.slide): \(error.localizedDescription)")
                return
            }
            if let count = (value as? NSNumber)?.intValue, count >= 0 {
                self.finishFailure(count == 0
                    ? "no slides found (expected [data-screen-label] or .slide elements)"
                    : "slide \(self.options.slide) does not exist; the deck has \(count) slides")
                return
            }
            next()
        }
    }

    /// An offscreen web view does not advance animations in real time, so a
    /// requested moment is reached by seeking every animation clock the page
    /// exposes: the artifact's own `window.pdSeek(ms)` when it defines one,
    /// otherwise CSS animations and transitions (Web Animations), GSAP
    /// (including paused timelines), anime.js and media. requestAnimationFrame loops that read the wall clock cannot
    /// be seeked; the reported count lets the caller tell what moved.
    private func seekIfRequested(then next: @escaping () -> Void, waited: TimeInterval = 0) {
        guard options.at > 0 else { next(); return }
        // A page that initializes asynchronously defines pdSeek once it is
        // ready; seeking earlier would be undone by its own setup.
        if waited < pdSeekWait {
            webView.evaluateJavaScript("typeof window.pdSeek === 'function'") { [weak self] value, _ in
                guard let self = self, !self.finished else { return }
                if (value as? Bool) == true {
                    self.seekIfRequested(then: next, waited: pdSeekWait)
                } else {
                    DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) { [weak self] in
                        self?.seekIfRequested(then: next, waited: waited + 0.1)
                    }
                }
            }
            return
        }
        let ms = Int((options.at * 1000).rounded())
        let script = """
        (function (ms) {
          var n = 0;
          // The artifact's own clock wins: it can also redraw canvases and players.
          try { if (typeof window.pdSeek === 'function') { window.pdSeek(ms); return 1; } } catch (e) {}
          try { document.getAnimations().forEach(function (a) { try { a.pause(); a.currentTime = ms; n++; } catch (e) {} }); } catch (e) {}
          try { if (window.gsap && gsap.globalTimeline) {
            gsap.globalTimeline.pause(); gsap.globalTimeline.seek(ms / 1000, false); n++;
            // Paused timelines (a film's own player) ignore the global clock.
            gsap.globalTimeline.getChildren(false, false, true).forEach(function (tl) { if (tl.paused()) { tl.seek(ms / 1000, false); n++; } });
          } } catch (e) {}
          try { if (window.anime && anime.running) anime.running.slice().forEach(function (a) { a.pause(); a.seek(ms); n++; }); } catch (e) {}
          try { document.querySelectorAll('video, audio').forEach(function (m) { m.pause(); m.currentTime = ms / 1000; n++; }); } catch (e) {}
          return n;
        })(\(ms))
        """
        webView.evaluateJavaScript(script) { [weak self] value, error in
            guard let self = self, !self.finished else { return }
            if let error = error {
                self.finishFailure("could not seek to \(ms) ms: \(error.localizedDescription)")
                return
            }
            self.seeked = (value as? NSNumber)?.intValue ?? 0
            next()
        }
    }

    // MARK: Measure / resize

    private func measure() {
        guard !finished else { return }
        webView.evaluateJavaScript(Scripts.measure) { [weak self] value, error in
            guard let self = self, !self.finished else { return }
            if let error = error {
                self.finishFailure("measure failed: \(error.localizedDescription)")
                return
            }
            guard let dict = value as? [String: Any] else {
                self.finishFailure("measure returned no data")
                return
            }

            let scrollWidth = (dict["scrollWidth"] as? NSNumber)?.doubleValue ?? 0
            let scrollHeight = (dict["scrollHeight"] as? NSNumber)?.doubleValue ?? 0
            let title = dict["title"] as? String ?? ""
            let errors = (dict["errors"] as? [Any])?.map { String(describing: $0) } ?? []
            let brokenImages = (dict["brokenImages"] as? [Any])?.map { String(describing: $0) } ?? []

            if self.options.fullPage {
                let target = min(
                    max(Double(self.options.height), scrollHeight),
                    Double(maxFullPageHeight)
                )
                let targetHeight = Int(target.rounded(.up))
                if targetHeight > self.options.height {
                    self.captureScrolled(
                        totalHeight: targetHeight,
                        scrollWidth: scrollWidth,
                        scrollHeight: scrollHeight,
                        errors: errors,
                        brokenImages: brokenImages,
                        title: title
                    )
                    return
                }
            }

            self.snapshot(
                scrollWidth: scrollWidth,
                scrollHeight: scrollHeight,
                errors: errors,
                brokenImages: brokenImages,
                title: title
            )
        }
    }

    // MARK: Full page

    /// Scrolls through the page so IntersectionObserver and scroll-linked
    /// reveals fire, returns to the top, then stitches one snapshot per
    /// viewport. Fixed and sticky elements appear once, in the first tile.
    private func captureScrolled(totalHeight: Int, scrollWidth: Double, scrollHeight: Double,
                                 errors: [String], brokenImages: [String], title: String) {
        let viewport = options.height
        let reveal = """
        for (var y = 0; y < total; y += step) {
          window.scrollTo({ top: y, left: 0, behavior: 'instant' });
          window.dispatchEvent(new Event('scroll'));
          await new Promise(function (r) { setTimeout(r, \(Int(revealStepDelay * 1000))); });
        }
        window.scrollTo({ top: 0, left: 0, behavior: 'instant' });
        window.dispatchEvent(new Event('scroll'));
        return true;
        """
        webView.callAsyncJavaScript(
            reveal,
            arguments: ["total": totalHeight, "step": viewport],
            in: nil,
            in: .page
        ) { [weak self] result in
            guard let self = self, !self.finished else { return }
            if case .failure(let error) = result {
                self.finishFailure("full-page scroll failed: \(error.localizedDescription)")
                return
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + revealSettleDelay) { [weak self] in
                guard let self = self else { return }
                var offsets: [Int] = []
                var y = 0
                while y < totalHeight {
                    offsets.append(min(y, totalHeight - viewport))
                    y += viewport
                }
                self.captureTiles(offsets: offsets, tiles: [], totalHeight: totalHeight) { [weak self] tiles in
                    guard let self = self else { return }
                    guard let png = Renderer.stitch(tiles, width: self.options.width, height: totalHeight) else {
                        self.finishFailure("could not encode PNG")
                        return
                    }
                    self.write(png) {
                        self.finishSuccess(
                            width: self.options.width,
                            height: totalHeight,
                            scrollWidth: scrollWidth,
                            scrollHeight: scrollHeight,
                            errors: errors,
                            brokenImages: brokenImages,
                            title: title
                        )
                    }
                }
            }
        }
    }

    private func captureTiles(offsets: [Int], tiles: [(Int, NSImage)], totalHeight: Int,
                              then done: @escaping ([(Int, NSImage)]) -> Void) {
        guard !finished else { return }
        guard let offset = offsets.first else { done(tiles); return }
        let hideChrome = tiles.isEmpty ? "" : """
        Array.prototype.forEach.call(document.body ? document.body.getElementsByTagName('*') : [], function (el) {
          var p = getComputedStyle(el).position;
          if (p === 'fixed' || p === 'sticky') el.style.setProperty('visibility', 'hidden', 'important');
        });
        """
        let script = """
        (function () {
          \(hideChrome)
          window.scrollTo({ top: \(offset), left: 0, behavior: 'instant' });
          window.dispatchEvent(new Event('scroll'));
          return Math.round(window.scrollY);
        })()
        """
        webView.evaluateJavaScript(script) { [weak self] value, error in
            guard let self = self, !self.finished else { return }
            if let error = error {
                self.finishFailure("full-page scroll failed: \(error.localizedDescription)")
                return
            }
            // The document may end before the measured height; place the tile
            // where the page actually scrolled.
            let actual = (value as? NSNumber)?.intValue ?? offset
            DispatchQueue.main.asyncAfter(deadline: .now() + tileDelay) { [weak self] in
                guard let self = self, !self.finished else { return }
                let configuration = WKSnapshotConfiguration()
                configuration.snapshotWidth = NSNumber(value: self.options.width)
                self.webView.takeSnapshot(with: configuration) { [weak self] image, error in
                    guard let self = self, !self.finished else { return }
                    guard let image = image, error == nil, image.size.width > 0 else {
                        self.finishFailure("snapshot failed: \(error?.localizedDescription ?? "empty image")")
                        return
                    }
                    self.captureTiles(
                        offsets: Array(offsets.dropFirst()),
                        tiles: tiles + [(min(actual, totalHeight - self.options.height), image)],
                        totalHeight: totalHeight,
                        then: done
                    )
                }
            }
        }
    }

    /// Draws viewport tiles at their scroll offsets into one width×height PNG.
    private static func stitch(_ tiles: [(Int, NSImage)], width: Int, height: Int) -> Data? {
        guard width > 0, height > 0, let first = tiles.first else { return nil }
        guard let rep = NSBitmapImageRep(
            bitmapDataPlanes: nil,
            pixelsWide: width,
            pixelsHigh: height,
            bitsPerSample: 8,
            samplesPerPixel: 4,
            hasAlpha: true,
            isPlanar: false,
            colorSpaceName: .deviceRGB,
            bytesPerRow: 0,
            bitsPerPixel: 0
        ) else { return nil }
        rep.size = NSSize(width: width, height: height)
        guard let context = NSGraphicsContext(bitmapImageRep: rep) else { return nil }
        NSGraphicsContext.saveGraphicsState()
        NSGraphicsContext.current = context
        context.imageInterpolation = .high
        NSColor.white.setFill()
        NSRect(x: 0, y: 0, width: width, height: height).fill()
        let tileHeight = first.1.size.height * CGFloat(width) / first.1.size.width
        for (offset, image) in tiles {
            // AppKit's origin is bottom-left.
            let y = CGFloat(height - offset) - tileHeight
            image.draw(in: NSRect(x: 0, y: y, width: CGFloat(width), height: tileHeight),
                       from: .zero, operation: .sourceOver, fraction: 1)
        }
        context.flushGraphics()
        NSGraphicsContext.restoreGraphicsState()
        return rep.representation(using: .png, properties: [:])
    }

    private func write(_ png: Data, then next: () -> Void) {
        let outURL = URL(fileURLWithPath: options.outPath)
        do {
            try FileManager.default.createDirectory(
                at: outURL.deletingLastPathComponent(),
                withIntermediateDirectories: true
            )
            try png.write(to: outURL, options: .atomic)
        } catch {
            finishFailure("could not write PNG: \(error.localizedDescription)")
            return
        }
        next()
    }

    // MARK: Snapshot

    private func snapshot(scrollWidth: Double, scrollHeight: Double,
                          errors: [String], brokenImages: [String], title: String) {
        guard !finished else { return }
        logWindowState("before snapshot")

        let configuration = WKSnapshotConfiguration()
        configuration.snapshotWidth = NSNumber(value: options.width)

        webView.takeSnapshot(with: configuration) { [weak self] image, error in
            guard let self = self, !self.finished else { return }
            if let error = error {
                self.finishFailure("snapshot failed: \(error.localizedDescription)")
                return
            }
            guard let image = image, image.size.width > 0, image.size.height > 0 else {
                self.finishFailure("snapshot returned an empty image")
                return
            }

            let width = self.options.width
            let height = self.options.height
            guard let png = Renderer.pngData(from: image, width: width, height: height) else {
                self.finishFailure("could not encode PNG")
                return
            }
            self.write(png) {
                self.finishSuccess(
                    width: width,
                    height: height,
                    scrollWidth: scrollWidth,
                    scrollHeight: scrollHeight,
                    errors: errors,
                    brokenImages: brokenImages,
                    title: title
                )
            }
        }
    }

    /// Draws the snapshot into a bitmap of exactly width×height pixels so the
    /// PNG is 1 device pixel per CSS pixel regardless of display backing scale.
    private static func pngData(from image: NSImage, width: Int, height: Int) -> Data? {
        guard width > 0, height > 0 else { return nil }
        guard let rep = NSBitmapImageRep(
            bitmapDataPlanes: nil,
            pixelsWide: width,
            pixelsHigh: height,
            bitsPerSample: 8,
            samplesPerPixel: 4,
            hasAlpha: true,
            isPlanar: false,
            colorSpaceName: .deviceRGB,
            bytesPerRow: 0,
            bitsPerPixel: 0
        ) else { return nil }
        rep.size = NSSize(width: width, height: height)
        guard let context = NSGraphicsContext(bitmapImageRep: rep) else { return nil }

        NSGraphicsContext.saveGraphicsState()
        NSGraphicsContext.current = context
        context.imageInterpolation = .high
        let rect = NSRect(x: 0, y: 0, width: width, height: height)
        NSColor.white.setFill()
        rect.fill()
        image.draw(in: rect, from: .zero, operation: .sourceOver, fraction: 1)
        context.flushGraphics()
        NSGraphicsContext.restoreGraphicsState()

        return rep.representation(using: .png, properties: [:])
    }

    // MARK: Diagnostics

    private func logWindowState(_ phase: String) {
        guard debugEnabled else { return }
        let onScreen = (CGWindowListCopyWindowInfo(
            [.optionOnScreenOnly, .excludeDesktopElements],
            kCGNullWindowID
        ) as? [[String: Any]]) ?? []
        let pid = ProcessInfo.processInfo.processIdentifier
        let mine = onScreen.filter { ($0[kCGWindowOwnerPID as String] as? Int32) == pid }
        let screens = NSScreen.screens.map { $0.frame }
        let details = mine.compactMap { info -> String? in
            guard let raw = info[kCGWindowBounds as String] as? [String: Any],
                  let bounds = CGRect(dictionaryRepresentation: raw as CFDictionary) else {
                return nil
            }
            let onDisplay = screens.contains { $0.intersects(bounds) }
            return "windowBounds=\(bounds) intersectsDisplay=\(onDisplay)"
        }.joined(separator: " ")
        let app = NSApplication.shared
        let message = """
        \(phase): frame=\(window.frame) visible=\(window.isVisible) \
        occlusion=\(window.occlusionState.rawValue) appActive=\(app.isActive) \
        activationPolicy=\(app.activationPolicy().rawValue) \
        onScreenWindowsForPid=\(mine.count) pid=\(pid) \(details)
        """
        FileHandle.standardError.write(Data(("[pidesigner-render] " + message + "\n").utf8))
    }
}

// MARK: - Application

private final class AppDelegate: NSObject, NSApplicationDelegate {
    private let options: RenderOptions
    private var renderer: Renderer?

    init(options: RenderOptions) {
        self.options = options
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        let renderer = Renderer(options: options)
        self.renderer = renderer
        renderer.start()
    }
}

private let parsedOptions = parseOptions()
private let application = NSApplication.shared
application.setActivationPolicy(.prohibited)
private let appDelegate = AppDelegate(options: parsedOptions)
application.delegate = appDelegate
application.run()
