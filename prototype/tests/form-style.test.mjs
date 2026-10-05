import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';

const css=readFileSync(new URL('../styles.css',import.meta.url),'utf8');

test('创建密钥表单的普通字段与开关字段使用同一标签层级',()=>{
 assert.match(css,/\.compact-key-form \.label,\.compact-key-form \.toggle-row>span\{font-size:12px;font-weight:500;line-height:1\.4;color:var\(--secondary\)\}/);
});
