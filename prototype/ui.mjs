export const esc=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
export const money=n=>'$'+Number(n||0).toFixed(2);
export const rmb=n=>'¥'+Number(n||0).toFixed(2);
export const num=n=>new Intl.NumberFormat('en-US').format(n||0);
export const compact=n=>n>=1e6?(n/1e6).toFixed(2)+'M':n>=1000?(n/1000).toFixed(1)+'K':String(n||0);
export const date=n=>new Date(n).toLocaleString('sv-SE',{hour12:false}).slice(0,16);
export const icon=(name,extra='')=>`<img src="./assets/${name}.svg" class="icon ${extra}" alt="" aria-hidden="true">`;
export const providerLogo=(provider,extra='')=>{const slug={OpenAI:'openai',Anthropic:'anthropic',xAI:'xai',DeepSeek:'deepseek'}[provider];return slug?`<img src="./assets/provider-${slug}.svg" class="provider-logo ${extra}" alt="" aria-hidden="true">`:'';};
const multiplierLabel=raw=>{if(raw==null||raw==='')return '—';const value=Number(raw);return Number.isFinite(value)?`${Number.isInteger(value)?value.toFixed(1):value}x倍率`:'—';};
export const routeIdentity=(route,extra='')=>route?`<span class="route-identity ${extra}">${providerLogo(route.provider,'route-provider-logo')}<strong>${esc(route.name)}</strong><span class="route-choice-multiplier data">${multiplierLabel(route.multiplier)}</span></span>`:'<span class="muted">选择线路</span>';
export const btn=(text,action,cls='',attrs='')=>`<button type="button" class="${cls}" data-action="${action}" ${attrs}>${text}</button>`;
export const iconBtn=(name,action,label,attrs='')=>btn(icon(name),action,'icon-btn',`aria-label="${esc(label)}" title="${esc(label)}" ${attrs}`);
export function dropdown({mode='simple',className='',trigger,menu}){return `<div class="brand-dropdown brand-dropdown-${mode}${className?' '+className:''}" data-dropdown>${trigger}${menu}</div>`;}
export function select(name,value,options,attrs=''){
 const items=options.map(o=>Array.isArray(o)?o:[o,o]);
 const current=items.find(([v])=>String(v)===String(value))||items[0];
 const native=`<select class="brand-dropdown-native" tabindex="-1" aria-hidden="true" name="${esc(name)}" ${attrs}>${items.map(([v,t])=>`<option value="${esc(v)}" ${String(value)===String(v)?'selected':''}>${esc(t)}</option>`).join('')}</select>`;
 const trigger=btn(`${esc(current?.[1]||'')}<span class="brand-dropdown-chevron" aria-hidden="true"></span>`,'dropdown-toggle','brand-dropdown-trigger',`aria-label="${esc(name)}：${esc(current?.[1]||'')}" aria-haspopup="listbox" aria-expanded="false"`);
 const menu=`<div class="brand-dropdown-menu" role="listbox" aria-label="${esc(name)}" hidden>${items.map(([v,t])=>btn(esc(t),'dropdown-choose','brand-dropdown-option',`data-value="${esc(v)}" role="option" aria-selected="${String(value)===String(v)}"`)).join('')}</div>`;
 return dropdown({trigger:native+trigger,menu});
}
export const field=(label,html,hint='')=>`<${html.includes('class="brand-dropdown ') ? 'div':'label'} class="field"><span class="label">${esc(label)}</span>${html}${hint?`<span class="hint">${hint}</span>`:''}</${html.includes('class="brand-dropdown ') ? 'div':'label'}>`;
export const empty=(title,action='')=>`<div class="empty"><p>${esc(title)}</p>${action}</div>`;
export const requestError=(title,message,retryAction)=>`<div class="request-error" role="alert"><div><strong>${esc(title)}</strong><p>${esc(message)}</p></div>${retryAction?btn('重试',retryAction,'small'):''}</div>`;
export const errorBox=()=>'<div class="form-error" role="alert" tabindex="-1"></div>';
export const formActions=(label='保存')=>`<footer class="modal-actions">${btn('取消','close')}<button type="submit" class="primary">${esc(label)}</button></footer>`;
let toastTimer,opener;
export function toast(text,isError=false){const t=document.querySelector('#toast');t.textContent=text;t.className='show'+(isError?' error':'');clearTimeout(toastTimer);toastTimer=setTimeout(()=>t.className='',3300);}
export function showModal(title,html,wide=false){const d=document.querySelector('#modal');if(!d.open)opener=document.activeElement;d.className=wide?'wide':'';d.innerHTML=`<header class="modal-head"><h2 id="modal-title">${esc(title)}</h2>${btn('×','close','icon-btn close-btn','aria-label="关闭弹窗"')}</header>${html}`;if(!d.open)d.showModal();d.scrollTop=0;queueMicrotask(()=>{const f=d.querySelector('[autofocus]')||d.querySelector('input:not([type=hidden]),select,textarea')||d.querySelector('button');f?.focus({preventScroll:true});});}
export function closeModal(){const d=document.querySelector('#modal');d.close();d.innerHTML='';opener?.isConnected&&opener.focus({preventScroll:true});}
export function formError(message){const e=document.querySelector('#modal .form-error');if(e){e.textContent=message;e.focus();}else toast(message,true);}
export async function copy(text){try{await navigator.clipboard.writeText(text);toast('已复制');return true;}catch{showModal('手动复制',`<div class="modal-body"><p class="secondary">浏览器未允许自动复制，请选中并复制下方内容。</p><textarea readonly aria-label="复制内容">${esc(text)}</textarea></div><footer class="modal-actions">${btn('关闭','close')}</footer>`);document.querySelector('#modal textarea').select();return false;}}
export function download(name,content,type='text/plain;charset=utf-8'){const blob=new Blob([content],{type}),url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(url),2000);}
export function table(headers,rows,cls='',labels=[]){return `<div class="table-scroll"><table class="${cls}"><thead><tr>${headers.map(h=>`<th scope="col">${h}</th>`).join('')}</tr></thead><tbody>${rows.map(cells=>`<tr>${cells.map((c,i)=>`<td${labels[i]?` data-label="${esc(labels[i])}"`:''}>${c}</td>`).join('')}</tr>`).join('')}</tbody></table></div>`;}
