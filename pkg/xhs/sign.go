package xhs

import "fmt"

// noteExtractJS returns JavaScript that reads the current note page's
// __INITIAL_STATE__ and returns a compact JSON object describing the note.
// Synchronous (the SSR state is already present on the page). Runs via a plain
// Runtime.evaluate (no awaitPromise needed), but works under awaitPromise too.
func noteExtractJS() string {
	return `(function(){
		var out={};
		try {
			var st=window.__INITIAL_STATE__;
			if (st && st.note && st.note.noteDetailMap){
				var k=Object.keys(st.note.noteDetailMap)[0];
				if (k){
					var n=st.note.noteDetailMap[k].note || st.note.noteDetailMap[k];
					if (n){
						out={
							id: n.noteId||n.id||k,
							title: n.title||'',
							desc: n.desc||'',
							author: (n.user&&n.user.nickname)||'',
							type: n.type||'',
							liked: (n.interactInfo&&n.interactInfo.likedCount)||'',
							comment: (n.interactInfo&&n.interactInfo.commentCount)||'',
							tags: (n.tagList?n.tagList.map(function(x){return x.name||'';}).filter(Boolean):[])
						};
					} else { out={error:'note_detail_empty'}; }
				} else { out={error:'no_note_in_map'}; }
			} else { out={error:'no_initial_state'}; }
		} catch(e){ out={error:e.message}; }
		return JSON.stringify(out);
	})()`
}

// searchFeedsJS returns SYNCHRONOUS JavaScript that reads the search results
// already loaded into __INITIAL_STATE__.search on a XHS search_result page.
//
// Why synchronous / page-state instead of an in-page fetch: Obscura's CDP
// server does not pump the page's event loop between Runtime.evaluate calls,
// so async work scheduled via Eval (fetch / setTimeout / Promise resolution)
// never progresses. But the page's OWN async runs to completion during
// navigation (the SPA boots, _webmsxyw loads, the search XHR fires). So we
// navigate to the search_result page, let the page's own (correctly signed)
// search request fire during load + scrolls, then read the populated state
// synchronously.
//
// Two result sources are returned:
//   - notes: from __INITIAL_STATE__.search.feeds (camelCase + note_card snake_case)
//   - domNotes: scraped from rendered <a href="/explore/<id>?xsec_token=..."> links
//
// The DOM scrape is a fallback for when the SPA renders cards into the DOM but
// the state array lags behind (or vice versa). Whichever is non-empty wins.
//
// Diagnostic fields (searchValue/hasMore/firstEnter/domCards) let the caller
// tell "search fired, no results" from "search never fired".
func searchFeedsJS() string {
	return `(function(){
		var out={count:0, notes:[], domCards:0, domNotes:[], href:location.href, hasState:false};
		try {
			// DOM note-card links: /explore/<id>?xsec_token=...
			var links=document.querySelectorAll('a[href*="/explore/"]');
			var seen={}; var dn=[];
			for (var i=0;i<links.length && dn.length<20;i++){
				var h=links[i].getAttribute('href')||'';
				if (h.indexOf('xsec_token')<0) continue;
				var m=h.match(/\/(?:explore|discovery\/item|note)\/([a-zA-Z0-9]+)/);
				var tm=h.match(/xsec_token=([^&]+)/);
				if (m && !seen[m[1]]){ seen[m[1]]=1; dn.push({id:m[1], xsec_token: tm?decodeURIComponent(tm[1]):''}); }
			}
			out.domCards=dn.length; out.domNotes=dn;

			var st=window.__INITIAL_STATE__;
			if (!st){ out.pageTextHead=(document.body?document.body.innerText:'').slice(0,160); return JSON.stringify(out); }
			out.hasState=true;
			var s = st.search || (st.page && st.page.search);
			if (!s){ out.searchMissing=true; out.stateKeys=Object.keys(st).slice(0,30); out.pageTextHead=(document.body?document.body.innerText:'').slice(0,160); return JSON.stringify(out); }
			out.searchKeys=Object.keys(s);
			out.searchValue=s.searchValue||'';
			out.hasMore=s.hasMore;
			out.firstEnter=s.firstEnterSearchPage;
			var feeds = s.feeds || s.notes || s.items || s.noteList || [];
			out.feedsType=(Array.isArray(feeds)?'array':typeof feeds);
			out.count = Array.isArray(feeds)?feeds.length:0;
			out.notes = (Array.isArray(feeds)?feeds:[]).slice(0,20).map(function(n){
				var nc = n.note_card || n.noteCard || null;
				return {
					id: n.id||n.noteId||(nc&&nc.note_id)||'',
					title: n.title||n.displayTitle||(nc&&nc.display_title)||'',
					xsec_token: n.xsec_token||'',
					type: n.type||(nc&&nc.type)||'',
					author: (n.user&&n.user.nickname)||(nc&&nc.user&&nc.user.nickname)||'',
					liked: (n.interactInfo&&n.interactInfo.likedCount)||(nc&&nc.interact_info&&nc.interact_info.liked_count)||''
				};
			}).filter(function(n){ return n.id; });
			out.pageTextHead=(document.body?document.body.innerText:'').slice(0,160);
		} catch(e){ out.error=e.message; }
		return JSON.stringify(out);
	})()`
}

// searchSubmitJS returns SYNCHRONOUS JavaScript that forces a client-side
// search by setting the search input's value and dispatching an Enter keydown.
// XHS submits search on Enter (onKeyDown); a plain el.click() on the search
// button does not reliably trigger React's handler, so we synthesize the
// keyboard event. The native value setter is used so React's controlled input
// picks up the new value, then 'input'/'change' + keydown/keypress/keyup Enter
// are dispatched. The subsequent search XHR is the page's OWN async and
// completes during the caller's polling loop.
func searchSubmitJS(keyword string) string {
	return fmt.Sprintf(`(function(){
		var inp = document.querySelector('input[placeholder*="搜索"]')
			|| document.querySelector('#search-input')
			|| document.querySelector('input.search-input')
			|| document.querySelector('input');
		if (!inp) return JSON.stringify({error:'no_search_input'});
		try {
			inp.focus();
			var proto = (inp.tagName==='TEXTAREA') ? window.HTMLTextAreaElement.prototype : window.HTMLInputElement.prototype;
			var setter = Object.getOwnPropertyDescriptor(proto, 'value').set;
			if (setter) setter.call(inp, %q);
			else inp.value = %q;
			inp.dispatchEvent(new Event('input', {bubbles:true}));
			inp.dispatchEvent(new Event('change', {bubbles:true}));
			var ev = function(t){
				try { return new KeyboardEvent(t, {key:'Enter', code:'Enter', keyCode:13, which:13, bubbles:true, cancelable:true}); }
				catch(e){ var k=document.createEvent('KeyboardEvent'); k.initKeyboardEvent(t,true,true,window,'Enter',13,'',false,''); return k; }
			};
			inp.dispatchEvent(ev('keydown'));
			inp.dispatchEvent(ev('keypress'));
			inp.dispatchEvent(ev('keyup'));
			return JSON.stringify({ok:true, value: inp.value, placeholder: inp.placeholder});
		} catch(e){ return JSON.stringify({error:e.message}); }
	})()`, keyword, keyword)
}

// scrollPageJS scrolls the page down by one viewport to trigger lazy-loaded
// search results. Synchronous.
func scrollPageJS() string {
	return `window.scrollBy(0, window.innerHeight); 'scrolled'`
}

// searchWaitJS returns an async IIFE that dispatches Enter on the search input
// (to trigger the SPA's own signed search) and then polls __INITIAL_STATE__-
// .search.feeds and the DOM note-card links until results appear or ~15s elapse.
//
// This MUST run via EvalAsync (awaitPromise): the awaited setTimeout calls pump
// the page event loop, which (unlike plain synchronous Eval) lets the SPA's
// post-hydration search XHR actually fire and complete. The SPA signs the
// request correctly itself, so this sidesteps the in-page signing/CORS problems
// of searchFireJS.
// searchSetupJS is SYNCHRONOUS JS (run via Eval) that (1) idempotently patches
// window.fetch and XMLHttpRequest to capture any search/notes request the SPA
// makes - including its (correctly signed) response - into window.__xhsCap, and
// (2) focuses the search input and sets its keyword. The caller then sends a
// real Enter key (PressEnter) to trigger the SPA's search, then runs
// searchCaptureJS to pump the loop and read the captured response.
func searchSetupJS(keyword string) string {
	return fmt.Sprintf(`(function(){
		var kw = %q;
		if (!window.__xhsCap) {
			window.__xhsCap = {reqs:[], resp:'', code:0};
			var of = window.fetch;
			window.fetch = function(url, opts){
				var u = (typeof url==='string')?url:(url&&url.url)||'';
				var p = of.apply(this, arguments);
				if (u.indexOf('search')>=0 && u.indexOf('notes')>=0) {
					window.__xhsCap.reqs.push({url:u, method:(opts&&opts.method)||'GET', headerKeys: opts&&opts.headers?Object.keys(opts.headers):[], body:(opts&&opts.body||'').slice(0,400), transport:'fetch'});
					p.then(function(r){ r.clone().text().then(function(t){ window.__xhsCap.resp=t.slice(0,8000); window.__xhsCap.code=r.status; }).catch(function(){}); }).catch(function(){});
				}
				return p;
			};
			var oo=XMLHttpRequest.prototype.open, os=XMLHttpRequest.prototype.send, osh=XMLHttpRequest.prototype.setRequestHeader;
			XMLHttpRequest.prototype.open=function(m,u){ this.__u=u; this.__m=m; this.__h={}; return oo.apply(this,arguments); };
			XMLHttpRequest.prototype.setRequestHeader=function(k,v){ try{this.__h[k]=v;}catch(e){} return osh.apply(this,arguments); };
			XMLHttpRequest.prototype.send=function(b){ var self=this; if(self.__u && self.__u.indexOf('search')>=0 && self.__u.indexOf('notes')>=0){ window.__xhsCap.reqs.push({url:self.__u,method:self.__m,headerKeys:Object.keys(self.__h),body:(b||'').slice(0,400),transport:'xhr'}); self.addEventListener('load',function(){ try{window.__xhsCap.resp=self.responseText.slice(0,8000);window.__xhsCap.code=self.status;}catch(e){} }); } return os.apply(this,arguments); };
		}
		var focused = false, ae = '';
		try {
			var inp = document.querySelector('input[placeholder*="搜索"]') || document.querySelector('#search-input') || document.querySelector('input');
			if (inp) {
				inp.focus();
				var proto = inp.tagName==='TEXTAREA'?window.HTMLTextAreaElement.prototype:window.HTMLInputElement.prototype;
				var d = Object.getOwnPropertyDescriptor(proto,'value');
				if (d&&d.set) d.set.call(inp, kw); else inp.value=kw;
				inp.dispatchEvent(new Event('input',{bubbles:true}));
				inp.dispatchEvent(new Event('change',{bubbles:true}));
				focused = true;
			}
			ae = document.activeElement ? (document.activeElement.tagName+'|'+document.activeElement.placeholder) : '';
		} catch(e){ window.__xhsCap.trigErr=String(e); }
		return JSON.stringify({focused: focused, active: ae, value: kw});
	})()`, keyword)
}

// searchCaptureJS is ASYNC JS (run via EvalAsync) that pumps the event loop
// with one same-origin fetch (so the SPA's Enter-triggered search XHR, fired
// between the setup Eval and this call, actually completes) and returns the
// captured search response.
func searchCaptureJS() string {
	return `(async function(){
		try{ await fetch('https://www.xiaohongshu.com/explore',{credentials:'include'}); }catch(e){}
		var c = window.__xhsCap || {reqs:[]};
		return JSON.stringify({status: c.resp?'done':'timeout', code: c.code, body: c.resp||'', reqs: c.reqs, trigErr: c.trigErr||'', href: location.href, hasCap: !!window.__xhsCap});
	})()`
}

// xhsCapPatchJS is injected via Page.addScriptToEvaluateOnNewDocument so it runs
// BEFORE the page's own scripts on every navigation. It patches window.fetch
// and XMLHttpRequest to capture any search/notes request the SPA makes during
// load - including its URL, method, full headers (so we can see x-s-common and
// any other signed headers), body, and response.
//
// This is how we learn the exact request format the SPA uses (the signed-fetch
// fallback gets a gateway 500 because we can't otherwise replicate it).
func xhsCapPatchJS() string {
	return `window.__xhsCap={reqs:[],resp:'',code:0};
var __of=window.fetch;
window.fetch=function(url,opts){
  var u=(typeof url==='string')?url:(url&&url.url)||'';
  var p=__of.apply(this,arguments);
  if(u.indexOf('search')>=0||u.indexOf('/notes')>=0){
    try{var h={};if(opts&&opts.headers){for(var k in opts.headers)h[k]=String(opts.headers[k]).slice(0,90);}window.__xhsCap.reqs.push({url:u,method:(opts&&opts.method)||'GET',headers:h,body:(opts&&opts.body||'').slice(0,600),transport:'fetch'});}catch(e){}
    p.then(function(r){r.clone().text().then(function(t){window.__xhsCap.resp=t.slice(0,8000);window.__xhsCap.code=r.status;window.__xhsCap.respUrl=u;}).catch(function(){});}).catch(function(){});
  }
  return p;
};
var __oo=XMLHttpRequest.prototype.open,__os=XMLHttpRequest.prototype.send,__osh=XMLHttpRequest.prototype.setRequestHeader;
XMLHttpRequest.prototype.open=function(m,u){this.__u=u;this.__m=m;this.__h={};return __oo.apply(this,arguments);};
XMLHttpRequest.prototype.setRequestHeader=function(k,v){try{this.__h[k]=String(v).slice(0,90);}catch(e){}return __osh.apply(this,arguments);};
XMLHttpRequest.prototype.send=function(b){var self=this;if(self.__u&&(self.__u.indexOf('search')>=0||self.__u.indexOf('/notes')>=0)){window.__xhsCap.reqs.push({url:self.__u,method:self.__m,headers:self.__h,body:(b||'').slice(0,600),transport:'xhr'});self.addEventListener('load',function(){try{window.__xhsCap.resp=self.responseText.slice(0,8000);window.__xhsCap.code=self.status;window.__xhsCap.respUrl=self.__u;}catch(e){}});}return __os.apply(this,arguments);};`
}

// readCaptureJS is SYNCHRONOUS JS that returns the captured requests/response.
func readCaptureJS() string {
	return `(function(){var c=window.__xhsCap||{reqs:[]};return JSON.stringify({resp:c.resp||'',code:c.code,reqs:c.reqs,respUrl:c.respUrl||'',href:location.href});})()`
}

// captureAllPatchJS is an IDEMPOTENT fetch/XHR patch (via
// addScriptToEvaluateOnNewDocument) that records every signed request the SPA
// makes during load, focusing on the x-s-common header (which _webmsxyw does
// NOT return, but the SPA may set on its own XHRs). Idempotent per window so
// repeated subframe injections don't nest-wrap fetch (which caused
// RangeError: Maximum call stack size exceeded).
func captureAllPatchJS() string {
	return `if(!window.__xhsAllPatched){window.__xhsAllPatched=true;window.__xhsAllCap=[];
var __of=window.fetch;window.fetch=function(url,opts){try{var u=(typeof url==='string')?url:(url&&url.url)||'';var h=(opts&&opts.headers)||{};var xsc=h['x-s-common']||h['X-s-common']||h['X-S-Common']||'';if(xsc){window.__xhsAllCap.push({url:u.slice(0,80),xsc:xsc});}}catch(e){}return __of.apply(this,arguments);};
var __oo=XMLHttpRequest.prototype.open,__os=XMLHttpRequest.prototype.send,__osh=XMLHttpRequest.prototype.setRequestHeader;
XMLHttpRequest.prototype.open=function(m,u){this.__u=u;this.__h={};return __oo.apply(this,arguments);};
XMLHttpRequest.prototype.setRequestHeader=function(k,v){try{this.__h[k.toLowerCase()]=v;}catch(e){}return __osh.apply(this,arguments);};
XMLHttpRequest.prototype.send=function(b){try{var xsc=this.__h['x-s-common']||'';if(xsc){window.__xhsAllCap.push({url:String(this.__u).slice(0,80),xsc:xsc});}}catch(e){}return __os.apply(this,arguments);};}`
}

// readAllCaptureJS is SYNCHRONOUS JS that returns all captured signed requests.
func readAllCaptureJS() string {
	return `(function(){return JSON.stringify(window.__xhsAllCap||[]);})()`
}

// stealthPatchJS is idempotent anti-detection JS injected before navigation so
// it runs before the page's own scripts. Patches are CONDITIONAL: they only
// fire when the value is missing or looks headless, so this is safe on both
// the headed-Chrome container (real values left intact) and Obscura (which
// applies its own stealth). Covers navigator.webdriver, plugins, languages,
// the window.chrome object, and the permissions query.
func stealthPatchJS() string {
	return `if(!window.__xhsStealthPatched){window.__xhsStealthPatched=true;
try{if(navigator.webdriver){Object.defineProperty(navigator,'webdriver',{get:function(){return undefined;}});}}catch(e){}
try{if(!navigator.languages||!navigator.languages.length){Object.defineProperty(navigator,'languages',{get:function(){return ['zh-CN','zh','en-US','en'];}});}}catch(e){}
try{if(!navigator.plugins||!navigator.plugins.length){Object.defineProperty(navigator,'plugins',{get:function(){return [1,2,3,4,5];}});}}catch(e){}
try{if(!window.chrome){window.chrome={runtime:{},app:{},csi:function(){return {};},loadTimes:function(){return {};}};}}catch(e){}
try{if(navigator.permissions&&navigator.permissions.query){var __pq=navigator.permissions.query.bind(navigator.permissions);navigator.permissions.query=function(p){return p&&p.name==='notifications'?Promise.resolve({state:typeof Notification!=='undefined'?Notification.permission:'default'}):__pq(p);};}}catch(e){}
}`
}

// pumpJS is ASYNC JS (run via EvalAsync) that pumps the page event loop via a
// same-origin fetch. Called repeatedly to give xhsFingerprintV3's async
// initialization event-loop time to complete (Obscura freezes the loop between
// CDP calls, so async init only progresses during an awaited fetch).
func pumpJS() string {
	return `(async function(){try{await fetch('https://www.xiaohongshu.com/explore',{credentials:'include'});}catch(e){}return 'pumped';})()`
}

// signSyncJS is SYNCHRONOUS JS (run via Eval) that signs the search request
// with _webmsxyw and returns x-s, x-t, x-s-common (if the fingerprint upgrade
// took), plus a getV18() probe. Caller sends the signed request server-side.
func signSyncJS(keyword string) string {
	return fmt.Sprintf(`(function(){
		var kw = %q;
		var sid = 'xxxxxxxxxxxx4xxxyxxxxxxxxxxxxxxx'.replace(/[xy]/g, function(c){ var r=Math.random()*16|0; return (c==='x'?r:(r&0x3|0x8)).toString(16); });
		var path = '/api/sns/web/v1/search/notes';
		var body = JSON.stringify({keyword:kw, page:1, page_size:20, search_id:sid, sort:'general', note_type:0, search_type:'note'});
		var signFn = window._webmsxyw;
		if (typeof signFn !== 'function') return JSON.stringify({error:'no_signfn'});
		var sign;
		try { sign = signFn(path, body); } catch(e){ return JSON.stringify({error:'sign_error', msg:String(e)}); }
		if (!sign) return JSON.stringify({error:'sign_null'});
		var v18=''; try{ var v=window.xhsFingerprintV3.getV18(); v18=(v===undefined)?'undefined':JSON.stringify(v).slice(0,80); }catch(e){ v18='err:'+String(e).slice(0,80); }
		return JSON.stringify({
			xs: sign['x-s']||sign['X-s']||'',
			xt: String(sign['x-t']||sign['X-t']||''),
			xsc: sign['x-s-common']||sign['X-s-common']||'',
			body: body, path: path,
			signKeys: Object.keys(sign),
			v18: v18
		});
	})()`, keyword)
}

// signDiagJS is SYNCHRONOUS JS that dumps signing-related window globals,
// localStorage keys, and a sample _webmsxyw return, to locate where
// x-s-common (which _webmsxyw does NOT return) comes from.
func signDiagJS() string {
	return `(function(){
		var win = Object.keys(window).filter(function(k){return /sign|mns|common|xyw|webm|encrypt|getXS|xhs|b1b1|finger/i.test(k);});
		var ls = [];
		try { ls = Object.keys(localStorage); } catch(e){}
		var signRet = {};
		try { var s = window._webmsxyw('/api/sns/web/v1/search/notes', '{"keyword":"test"}'); if(s){ for(var k in s) signRet[k]=String(s[k]).slice(0,50); } else { signRet.null=true; } } catch(e){ signRet.err=String(e); }
		var fp = {};
		try {
			var f = window.xhsFingerprintV3;
			fp.type = typeof f;
			if (f && typeof f==='object') {
				fp.keys = Object.keys(f);
				try { fp.v18 = JSON.stringify(f.getV18()).slice(0, 300); } catch(e){ fp.v18Err=String(e); }
				try { fp.miniUa = JSON.stringify(f.getCurMiniUa()).slice(0, 300); } catch(e){ fp.uaErr=String(e); }
			}
		} catch(e){ fp.err=String(e); }
		var msrc=''; try{ msrc=String(window._webmsxyw).slice(0,800); }catch(e){ msrc='err:'+e; }
		return JSON.stringify({win:win, ls:ls, signRet:signRet, fp:fp, msrc:msrc, ua:navigator.userAgent});
	})()`
}

// searchFireJS returns an async IIFE that signs the XHS search API URL with
// _webmsxyw and awaits a GET fetch to the search API, returning the response
// (status + body) as a JSON string. Must be run via EvalAsync (awaitPromise)
// so the page event loop pumps and the fetch actually completes - a plain Eval
// leaves the loop frozen on Obscura and the fetch.then never fires.
func searchFireJS(keyword string) string {
	return fmt.Sprintf(`(async function(){
		var kw = %q;
		var sid = 'xxxxxxxxxxxx4xxxyxxxxxxxxxxxxxxx'.replace(/[xy]/g, function(c){ var r=Math.random()*16|0; return (c==='x'?r:(r&0x3|0x8)).toString(16); });
		var path = '/api/sns/web/v1/search/notes';
		var body = JSON.stringify({keyword:kw, page:1, page_size:20, search_id:sid, sort:'general', note_type:0, search_type:'note'});
		var signFn = window._webmsxyw;
		if (typeof signFn !== 'function') return JSON.stringify({status:'no_signfn'});
		var sign;
		try { sign = signFn(path, body); } catch(e){ return JSON.stringify({status:'sign_error', msg:String(e)}); }
		if (!sign) return JSON.stringify({status:'sign_null'});
		var signDump = {};
		Object.keys(sign).forEach(function(k){ signDump[k] = String(sign[k]).slice(0,60); });
		var xs = sign['x-s']||sign['X-s']||'';
		var xt = String(sign['x-t']||sign['X-t']||'');
		var xsc = sign['x-s-common']||sign['X-s-common']||'';
		if (!xs) return JSON.stringify({status:'no_xs', sign:signDump});
		var headers = {'x-s':xs, 'x-t':xt, 'x-s-common':xsc, 'Content-Type':'application/json'};
		try {
			var r = await fetch('https://www.xiaohongshu.com'+path, {method:'POST', headers:headers, body:body, credentials:'include'});
			var t = await r.text();
			return JSON.stringify({status:'done', code:r.status, body:t.slice(0,8000), sign:signDump});
		} catch(e){ return JSON.stringify({status:'fetch_error', msg:String(e), sign:signDump}); }
	})()`, keyword)
}
