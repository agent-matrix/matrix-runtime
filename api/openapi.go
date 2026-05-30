package api

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var openAPISpec []byte

// handleOpenAPISpec serves the raw OpenAPI document.
func (s *Server) handleOpenAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = w.Write(openAPISpec)
}

// docsHTML is a self-contained API docs viewer. It loads Redoc from a CDN when
// online, but falls back to a readable, dependency-free rendering of
// /openapi.yaml so the page works air-gapped too.
const docsHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width,initial-scale=1"/>
  <title>Matrix Runtime API — Docs</title>
  <style>
    body{margin:0;font-family:ui-sans-serif,system-ui,Segoe UI,Roboto,Arial;background:#0b0f14;color:#e6edf3}
    header{padding:18px 24px;border-bottom:1px solid #1b2430;display:flex;align-items:center;gap:12px}
    header h1{font-size:16px;margin:0;font-weight:700}
    header .v{font-family:ui-monospace,monospace;font-size:11px;color:#7d8da1;border:1px solid #1b2430;border-radius:6px;padding:2px 8px}
    a{color:#5ad19a;text-decoration:none}
    #fallback{max-width:900px;margin:0 auto;padding:24px}
    .op{border:1px solid #1b2430;border-radius:10px;margin:10px 0;overflow:hidden}
    .op .h{display:flex;align-items:center;gap:10px;padding:11px 14px;background:#0f1620}
    .m{font-family:ui-monospace,monospace;font-size:11px;font-weight:700;padding:2px 8px;border-radius:5px;text-transform:uppercase}
    .get{background:#10331f;color:#5ad19a}.post{background:#102a44;color:#6db3f2}
    .put{background:#3a2f10;color:#e8c468}.delete{background:#3a1414;color:#f2796d}
    .path{font-family:ui-monospace,monospace;font-size:13px}
    .sum{color:#9fb0c3;font-size:12.5px;padding:8px 14px}
  </style>
</head>
<body>
  <header>
    <h1>Matrix Runtime API</h1><span class="v" id="ver">openapi</span>
    <span style="flex:1"></span><a href="/openapi.yaml">openapi.yaml ↗</a>
  </header>
  <redoc spec-url="/openapi.yaml"></redoc>
  <div id="fallback"><p style="color:#7d8da1">Rendering API reference…</p></div>
  <script src="https://cdn.redocly.com/redoc/latest/bundles/redoc.standalone.js"
          onload="document.getElementById('fallback').style.display='none'"
          onerror="renderFallback()"></script>
  <script>
    function renderFallback(){
      var rd=document.querySelector('redoc'); if(rd) rd.remove();
      fetch('/openapi.yaml').then(function(r){return r.text()}).then(function(t){
        // Minimal YAML-ish scan: list "  /path:" then "    get:" with summary.
        var lines=t.split('\n'), out=[], path=null, method=null, mLine=-1;
        var box=document.getElementById('fallback'); box.innerHTML='';
        var verEl=document.getElementById('ver');
        for(var i=0;i<lines.length;i++){
          var ln=lines[i];
          var pm=ln.match(/^  (\/[^:]+):\s*$/); if(pm){path=pm[1];continue;}
          var mm=ln.match(/^    (get|post|put|delete|patch):\s*$/);
          if(mm&&path){method=mm[1];
            var sum=''; for(var j=i+1;j<Math.min(i+6,lines.length);j++){var sm=lines[j].match(/^      summary:\s*(.+)$/); if(sm){sum=sm[1];break;}}
            var div=document.createElement('div'); div.className='op';
            div.innerHTML='<div class="h"><span class="m '+method+'">'+method+'</span><span class="path">'+path+'</span></div>'+(sum?'<div class="sum">'+sum+'</div>':'');
            box.appendChild(div);
          }
          var vm=ln.match(/^  version:\s*(.+)$/); if(vm&&verEl) verEl.textContent='v'+vm[1].trim();
        }
        if(!box.children.length) box.innerHTML='<p>See <a href="/openapi.yaml">/openapi.yaml</a>.</p>';
      });
    }
  </script>
</body>
</html>`

// handleDocs serves the API docs viewer.
func (s *Server) handleDocs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(docsHTML))
}
