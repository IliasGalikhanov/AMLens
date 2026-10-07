import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { getLocale, readLocale, setLocale, subscribe, t } from './core.ts';
import { createApiClient } from '../shared/api/client.ts';
import { roleLabels, formatScore } from '../features/workspace/labels.ts';
import { validateParquetFile } from '../features/workspace/importModel.ts';

test('English default and valid saved preferences, including unavailable storage', () => {
  assert.equal(getLocale(), 'en');
  assert.equal(readLocale(), 'en');
  assert.equal(readLocale({ getItem: () => 'fr' }), 'en');
  assert.equal(readLocale({ getItem: () => 'kk' }), 'kk');
  assert.equal(readLocale({ getItem() { throw Error('blocked'); } }), 'en');
});
test('language changes persist, notify subscribers and update labels and validation', async () => {
  const original = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
  const saved = new Map();
  Object.defineProperty(globalThis,'localStorage',{configurable:true,value:{setItem:(k,v)=>saved.set(k,v)}});
  let notifications=0;const unsubscribe=subscribe(()=>notifications++);
  try {
    setLocale('ru'); assert.equal(roleLabels.transit,'Транзит'); assert.equal(formatScore(.8),'0,80');
    assert.match(await validateParquetFile(new File([], 'nodes.parquet')),/пуст/);
    setLocale('kk'); assert.equal(t('Ваш вопрос'),'Сұрағыңыз'); assert.equal(roleLabels.coordinator,'Үйлестіру');
    assert.equal(saved.get('amlens.language'),'kk'); assert.equal(notifications,2);
    assert.match(await validateParquetFile(new File([], 'nodes.parquet')),/бос/);
    setLocale('en'); assert.equal(t('Загрузка графа…'),'Loading graph…'); assert.equal(formatScore(.8),'0.80');
    assert.equal(t('Ответ по клиенту {0} получен.', ['9007199254741001']),'Answer received for client 9007199254741001.');
  } finally { unsubscribe(); setLocale('en'); if(original)Object.defineProperty(globalThis,'localStorage',original);else delete globalThis.localStorage; }
});
test('every catalog entry has three translations and preserves interpolation placeholders', () => {
  const catalog=JSON.parse(readFileSync(new URL('./messages.json',import.meta.url),'utf8'));
  for(const [key,entry] of Object.entries(catalog)) {
    const placeholders=s=>[...s.matchAll(/\{\d+\}/g)].map(m=>m[0]).sort();
    for(const locale of ['en','ru','kk']) {
      assert.ok(entry[locale]?.trim(),key+' '+locale);
      assert.deepEqual(placeholders(entry[locale]),placeholders(entry.ru),key+' '+locale);
    }
  }
});
test('API requests carry the selected locale and retain JSON content type', async () => {
  let header;
  const api=createApiClient({fetcher:async (_url,init)=>{
    header=new Headers(init.headers).get('Accept-Language');
    return new Response(JSON.stringify({status:'ok',analysis_ready:false,ai_configured:false}));
  }});
  try {for(const locale of ['en','ru','kk']){setLocale(locale);await api.getHealth();assert.equal(header,locale);}}
  finally {setLocale('en');}
});
