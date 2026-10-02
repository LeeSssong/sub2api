import test from 'node:test';
import assert from 'node:assert/strict';
import {select} from '../ui.mjs';
import {page} from '../pages.mjs';
import * as M from '../model.mjs';

test('ordinary dropdown uses branded menu while preserving its change contract',()=>{
 const html=select('线路筛选','b',[['','全部线路'],['a','线路 A'],['b','线路 B']],'data-change="key-filter" data-field="route"');
 assert.match(html,/class="brand-dropdown brand-dropdown-simple"/);
 assert.match(html,/data-action="dropdown-toggle"[^>]*aria-expanded="false"/);
 assert.match(html,/class="brand-dropdown-menu"[^>]*role="listbox"[^>]*hidden/);
 assert.match(html,/data-action="dropdown-choose"[^>]*data-value="b"[^>]*aria-selected="true"/);
 assert.match(html,/<select[^>]*data-change="key-filter" data-field="route"/);
 assert.match(html,/>线路 B<\/button>/);
});

test('key route details share the branded dropdown surface',()=>{
 const state=M.createSeed(),key=state.keys[0];
 const html=page({state,page:'keys',keyFilter:{search:'',route:'',status:''},keySort:['name',1],keyPage:1,keyPageSize:20,keyRouteOpen:key.id,keyRouteSearch:'',keyRouteAnchor:{left:12,top:80,width:360,maxHeight:310}});
 assert.match(html,/class="brand-dropdown brand-dropdown-rich key-route-cell"/);
 assert.match(html,/class="brand-dropdown-menu key-route-popover"/);
 assert.match(html,/data-input="key-route-search"/);
 assert.match(html,/近 1 小时稳定性/);
});
