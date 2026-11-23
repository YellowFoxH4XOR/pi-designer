// Installed only in served live previews; never written into project files.
var stopPicking;
function pickElement(token, origin) {
  if (stopPicking) stopPicking();
  if (typeof token !== "string" || !token) return;
  var box = document.createElement("div");
  box.setAttribute("aria-hidden", "true");
  box.style.cssText = "position:fixed;display:none;z-index:2147483647;pointer-events:none;outline:2px solid #bd5033;background:rgba(189,80,51,.08)";
  document.documentElement.appendChild(box);
  function highlight(el) {
    if (!(el instanceof Element)) return;
    var r = el.getBoundingClientRect();
    Object.assign(box.style, {display:"block", left:r.left+"px", top:r.top+"px", width:r.width+"px", height:r.height+"px"});
  }
  function over(e) { highlight(e.target); }
  function locate(el) {
    var parts = [];
    for (; el && el.nodeType === 1; el = el.parentElement) {
      var tag = el.localName.toLowerCase(), index = 1;
      if (!/^[a-z][a-z0-9-]*$/.test(tag)) return "";
      for (var prev = el.previousElementSibling; prev; prev = prev.previousElementSibling)
        if (prev.localName === el.localName) index++;
      parts.unshift(tag + ":nth-of-type(" + index + ")");
    }
    return parts.join(" > ");
  }
  function finish(el) {
    // Only the structural address leaves the preview, never input values or page text.
    var selector = el ? locate(el) : "";
    stopPicking();
    parent.postMessage({source:"pidesigner-preview", type:"element-picked", token:token,
      pathname:location.pathname, selector:selector}, origin);
  }
  function block(e) { e.preventDefault(); e.stopImmediatePropagation(); }
  function click(e) { block(e); finish(e.target); }
  function key(e) {
    if (e.key === "Escape") { block(e); finish(null); }
    else if (e.key === "Enter") { block(e); finish(document.activeElement); }
  }
  var listeners = {pointerover:over, focusin:over, pointerdown:block, pointerup:block, mousedown:block, mouseup:block, click:click, keydown:key};
  stopPicking = function () {
    Object.keys(listeners).forEach(function (name) { window.removeEventListener(name, listeners[name], true); });
    box.remove(); stopPicking = null;
  };
  Object.keys(listeners).forEach(function (name) { window.addEventListener(name, listeners[name], true); });
}
