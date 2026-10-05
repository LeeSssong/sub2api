import test from 'node:test';
import assert from 'node:assert/strict';
import {routeIdentity} from '../ui.mjs';

test('线路身份统一显示后台倍率值加 x倍率，整数保留一位小数',()=>{
 const route={provider:'OpenAI',name:'示例线路',multiplier:1};
 assert.match(routeIdentity(route),/route-choice-multiplier data">1\.0x倍率<\/span>/);
 assert.match(routeIdentity({...route,multiplier:0.1}),/>0\.1x倍率<\/span>/);
 assert.match(routeIdentity({...route,multiplier:0.125}),/>0\.125x倍率<\/span>/);
});
