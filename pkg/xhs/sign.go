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
