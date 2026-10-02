import {esc,money,date,btn} from './ui.mjs';
import * as M from './model.mjs';

export const extraKeyHeaders=['ID','额度限制','速率限制','状态','最近使用','最近使用 IP','创建时间'];
export const defaultHiddenKeyColumns=[7,8,9,10,11,12,13];

export function extraKeyCells(key,usage){
 const spent=M.summarize(usage.filter(row=>row.time>(key.quotaResetAt||0))).cost;
 const latest=usage.length?usage.reduce((a,b)=>a.time>b.time?a:b):null;
 const status=M.keyStatus(key);
 const windows=[['5h','rate5h','usage5h'],['1d','rate1d','usage1d'],['7d','rate7d','usage7d']].filter(([,limit])=>key[limit]>0);
 const rateCell=windows.length?`<div class="key-rate-windows">${windows.map(([label,limit,used])=>`<span class="data">${label} ${money(key[used]||0)} / ${money(key[limit])}</span>`).join('')}${windows.some(([, ,used])=>key[used]>0)?btn('重置用量','reset-key-rate','small',`data-id="${esc(key.id)}"`):''}</div>`:'<span class="muted">无限制</span>';
 return [
  `<code>${esc(key.id)}</code>`,
  key.limit>0?`<span class="data">${money(spent)} / ${money(key.limit)}</span>${spent?btn('重置用量','reset-key-quota','small',`data-id="${esc(key.id)}"`):''}`:'<span class="muted">无限制</span>',
  rateCell,
  `<span class="${status==='enabled'?'success':'muted'}">${{enabled:'启用',disabled:'已禁用',expired:'已过期'}[status]}</span>`,
  latest?date(latest.time):'<span class="muted">尚未使用</span>',
  latest?.ip?esc(latest.ip):'<span class="muted">—</span>',
  date(key.created)
 ];
}
