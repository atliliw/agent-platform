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

// searchJS returns async JavaScript that calls the XHS search API from within
// the page so XHS's own JS signs the request. Strategy:
//
//  1. Plain fetch first - if XHS wraps window.fetch to auto-inject x-s/x-t,
//     the request is signed for free.
//  2. If that fails, locate the signing function (window._webmsxyw and
//     siblings) and sign manually, then fetch.
//
// Returns a JSON object {attempt, status, body?, signFn?, signedKeys?,
// candidates?, error?}. The Go side parses `body` (the raw API response, capped
// to keep returnByValue well under the size that hangs Obscura's CDP).
func searchJS(keyword string, page int, sort string) string {
	if page < 1 {
		page = 1
	}
	if sort == "" {
		sort = defaultSort
	}
	const tmpl = `(async function(){
		var keyword=%q;
		var page=%d;
		var sort=%q;
		var pageSize=20;
		var searchId=(crypto&&crypto.randomUUID)?crypto.randomUUID():('xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g,function(c){var r=Math.random()*16|0;var v=c=='x'?r:(r&0x3|0x8);return v.toString(16);}));
		var apiPath='/api/sns/web/v1/search/notes';
		var params='keyword='+encodeURIComponent(keyword)+'&page='+page+'&page_size='+pageSize+'&search_id='+searchId+'&sort='+sort+'&note_type=0';
		var fullUrl='https://edith.xiaohongshu.com'+apiPath+'?'+params;

		function findSign(){
			var keys=Object.keys(window);
			for (var i=0;i<keys.length;i++){
				var k=keys[i];
				if (/_webms|sign|xyw|mns/i.test(k) && typeof window[k]==='function') return {name:k, fn:window[k]};
			}
			return null;
		}
		function signCandidates(){
			return Object.keys(window).filter(function(k){return /_webms|sign|xyw|mns/i.test(k);}).slice(0,40);
		}

		// Attempt 1: plain fetch (rely on XHS fetch interceptor auto-signing).
		var plain={};
		try{
			var r1=await fetch(fullUrl,{method:'GET',credentials:'include'});
			var t1=await r1.text();
			plain={status:r1.status, body:t1.slice(0,50000)};
			if(r1.status===200 && t1.indexOf('"success":true')>=0){
				return JSON.stringify({attempt:'plain', status:r1.status, body:t1.slice(0,50000)});
			}
		}catch(e){ plain={error:e.message}; }

		// Attempt 2: explicit sign + fetch. Wait for the sign fn to bootstrap.
		var sign=null;
		for (var i=0;i<20 && !sign;i++){ sign=findSign(); if(!sign) await new Promise(function(r){setTimeout(r,500);}); }
		if(!sign){
			return JSON.stringify({attempt:'no_sign_function', plain:plain, candidates:signCandidates()});
		}
		var headers={};
		try{
			var s=sign.fn(apiPath+'?'+params, null);
			if(s){ for(var key in s){ headers[key]=s[key]; } }
		}catch(e){
			return JSON.stringify({attempt:'sign_call_failed', signFn:sign.name, plain:plain, detail:e.message});
		}
		if(!headers['x-s'] && !headers['X-s'] && !headers['x-s-common']){
			return JSON.stringify({attempt:'sign_returned_no_headers', signFn:sign.name, signedKeys:Object.keys(headers), plain:plain});
		}
		try{
			var r2=await fetch(fullUrl,{method:'GET',headers:headers,credentials:'include'});
			var t2=await r2.text();
			return JSON.stringify({attempt:'signed', status:r2.status, body:t2.slice(0,50000), signFn:sign.name, signedKeys:Object.keys(headers), plain:plain});
		}catch(e){
			return JSON.stringify({attempt:'signed_fetch_failed', signFn:sign.name, signedKeys:Object.keys(headers), plain:plain, detail:e.message});
		}
	})()`
	return fmt.Sprintf(tmpl, keyword, page, sort)
}
