<script setup lang="ts">
import {computed,onMounted,onUnmounted,ref} from 'vue';
import defaults from '../../config/ja.json';
import {rollingDamageSeries} from './dpsSeries';
import {resultCSV,type CombatLog,type Result} from './model';
const labels=ref<Record<string,string>>(defaults); const t=(key:string)=>labels.value[key]||key;
type Dictionary={name?:string;skills:Record<string,string>;classes:Record<string,string>;enemies:Record<string,string>};
const regions=ref<Record<string,Dictionary>>({}); const log=ref<CombatLog|null>(null); const result=ref<Result|null>(null);
const cursor=ref(0), idle=ref(5), speed=ref(1), playing=ref(false), busy=ref(false), settings=ref(false), error=ref(''), status=ref('');
const selectedBattle=ref(0),selectedPlayer=ref(''); let generation=0, timer:ReturnType<typeof setTimeout>|undefined;
const endTime=computed(()=>Math.min(604800000,(log.value?.events.at(-1)?.atMs??0)+idle.value*1000));
const battle=computed(()=>result.value?.battles[selectedBattle.value]);
const player=computed(()=>battle.value?.players.find(p=>p.id===selectedPlayer.value)??battle.value?.players[0]);
const dictionary=computed(()=>regions.value[log.value?.source==='synthetic'?'synthetic':log.value?.region??'jp']);
const label=(kind:'skills'|'classes'|'enemies',id:string)=>dictionary.value?.[kind]?.[id]??`${t(kind==='skills'?'unknownSkill':kind==='classes'?'unknownClass':'unknownEnemy')} (${id||'—'})`;
const fmt=(n:number)=>n.toLocaleString('ja-JP',{maximumFractionDigits:1}); const pct=(n:number)=>(n*100).toFixed(1)+'%';
const clock=(ms:number)=>`${String(Math.floor(ms/60000)).padStart(2,'0')}:${(ms/1000%60).toFixed(1).padStart(4,'0')}`;
const chart=computed(()=>{
 const b=battle.value;if(!b)return [];
 return rollingDamageSeries(b.damageEvents.map(h=>({At:h.atMs/1000,Damage:h.damage??0})),b.startMs/1000,5);
});
const points=computed(()=>{const values=chart.value; const max=Math.max(1,...values.map(p=>p[1])); const xmax=Math.max(1000,...values.map(p=>p[0])); return values.map(p=>`${(p[0]/xmax)*980+10},${130-p[1]/max*110}`).join(' ')});
async function json<T>(url:string,init?:RequestInit):Promise<T>{const response=await fetch(url,init); if(!response.ok)throw new Error(await response.text());return response.json();}
async function refresh(){
 if(!log.value)return; const token=++generation;busy.value=true;
 try{const data=await json<Result>('/api/analyze',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({log:log.value,untilMs:Math.round(cursor.value),idleMs:Math.round(idle.value*1000)})}); if(token!==generation)return;result.value=data;error.value='';if(selectedBattle.value>=data.battles.length)selectedBattle.value=Math.max(0,data.battles.length-1);}
 catch(e){if(token===generation){error.value=String(e instanceof Error?e.message:e);playing.value=false;}}
 finally{if(token===generation)busy.value=false;}
}
function pause(){playing.value=false;if(timer)clearTimeout(timer);}
async function load(data:CombatLog){pause();const token=++generation;busy.value=true;try{
 const until=Math.min(604800000,(data.events?.at(-1)?.atMs??0)+idle.value*1000);
 await json<Result>('/api/analyze',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({log:data,untilMs:until,idleMs:idle.value*1000})});
 if(token!==generation)return;log.value=data;cursor.value=0;selectedBattle.value=0;selectedPlayer.value='';result.value=null;status.value=t('loaded');await refresh();
 }catch(e){if(token===generation)error.value=e instanceof Error?e.message:t('importError');}finally{if(token===generation)busy.value=false;}}
async function sample(){try{await load(await json<CombatLog>('/api/sample'));}catch(e){error.value=String(e);}}
async function importFile(event:Event){const input=event.target as HTMLInputElement;const file=input.files?.[0];if(!file)return;try{if(file.size>8*1024*1024)throw new Error(t('importError'));await load(JSON.parse((await file.text()).replace(/^\ufeff/,'')));}catch(e){error.value=e instanceof Error?e.message:t('importError');}input.value='';}
async function tick(){if(!playing.value)return;cursor.value=Math.min(endTime.value,cursor.value+250*speed.value);await refresh();if(cursor.value>=endTime.value)playing.value=false;if(playing.value)timer=setTimeout(tick,250);}
function play(){if(!log.value)return;if(playing.value){pause();return}if(cursor.value>=endTime.value)cursor.value=0;playing.value=true;timer=setTimeout(tick,250);}
function reset(){pause();++generation;busy.value=false;log.value=null;result.value=null;cursor.value=0;selectedPlayer.value='';error.value='';status.value=t('resetDone');}
async function seek(){pause();await refresh();}
async function changeIdle(){pause();if(!Number.isFinite(idle.value)||idle.value<1||idle.value>60){idle.value=5;}if(log.value){cursor.value=Math.min(cursor.value,endTime.value);await refresh();}}
function download(kind:'json'|'csv'){if(!result.value)return;const value=kind==='json'?JSON.stringify(result.value,null,2):resultCSV(result.value);const blob=new Blob([value],{type:kind==='json'?'application/json;charset=utf-8':'text/csv;charset=utf-8'});const url=URL.createObjectURL(blob);const a=document.createElement('a');a.href=url;a.download=`mogugi-offline-${new Date().toISOString().replaceAll(':','-')}.${kind}`;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);}
onMounted(async()=>{try{const [ja,r]=await Promise.all([json<Record<string,string>>('/api/ja'),json<Record<string,Dictionary>>('/api/regions')]);labels.value={...defaults,...ja};regions.value=r;}catch{error.value=t('configError');}});
onUnmounted(()=>{pause();generation++;});
</script>
<template>
 <main>
  <header><div class="brand"><span class="mark">M</span><div><h1>{{t('title')}} <small>0.1</small></h1><p>{{t('subtitle')}}</p></div></div><span class="badge">OFFLINE</span></header>
  <aside class="notice">{{t('notice')}}</aside>
  <nav class="toolbar"><button class="primary" :disabled="busy" @click="sample">{{t('sample')}}</button><label class="button">{{t('import')}}<input type="file" accept=".json,application/json" :disabled="busy" @change="importFile"></label><button :disabled="!log || busy" @click="play">{{playing?t('pause'):t('play')}}</button><button @click="reset">{{t('reset')}}</button><span class="spacer"></span><button :disabled="!result?.battles.length || busy" @click="download('json')">{{t('save')}}</button><button :disabled="!result?.battles.length || busy" @click="download('csv')">{{t('csv')}}</button><button :aria-expanded="settings" @click="settings=!settings">{{t('settings')}}</button></nav>
  <div v-if="error" role="alert" class="error">{{t('error')}}：{{error}}</div><p class="status" role="status">{{status}}</p>
  <section v-if="settings" class="panel settings"><h2>{{t('settings')}}</h2><label>{{t('idle')}} <input type="number" v-model.number="idle" min="1" max="60" @change="changeIdle"></label><p>日本版・台湾地域の表示辞書は config/regions.json で管理します。地域は入力データの region で決まります。</p><p>{{t('formula')}}</p><p>{{t('stopHelp')}}</p></section>
  <template v-if="log">
   <div class="meta"><span>{{log.source==='synthetic'?t('synthetic'):t('offline')}}</span><span>{{regions[log.region]?.name??log.region}}</span></div>
   <section class="panel transport"><label>{{t('time')}} <strong>{{clock(cursor)}} / {{clock(endTime)}}</strong></label><input aria-label="再生時刻" type="range" min="0" :max="endTime" step="250" v-model.number="cursor" @input="seek"><label>{{t('speed')}} <select v-model.number="speed"><option :value="1">1×</option><option :value="2">2×</option><option :value="4">4×</option></select></label></section>
   <div class="battle-tabs"><button v-for="(b,i) in result?.battles" :key="b.number" :class="{selected:selectedBattle===i}" @click="selectedBattle=i;selectedPlayer=''">{{t('battle')}} {{b.number}} · {{b.active?t('active'):t('ended')}}</button></div>
   <template v-if="battle">
    <section class="metrics"><article><span>{{t('duration')}}</span><strong>{{clock(battle.durationSec*1000)}}</strong></article><article><span>{{t('total')}}</span><strong>{{fmt(battle.damage)}}</strong></article><article><span>{{t('party')}} DPS</span><strong>{{fmt(battle.damage/Math.max(1,battle.durationSec))}}</strong></article><article><span>{{t('battle')}} {{battle.number}}</span><strong class="state">{{battle.active?t('active'):battle.reason==='idle'?t('endIdle'):t('endExplicit')}}</strong></article></section>
    <section class="panel"><h2>{{t('player')}}{{t('battleDamage')}}</h2><div class="table-wrap"><table><thead><tr><th>{{t('player')}}</th><th>{{t('class')}}</th><th>{{t('dps')}}</th><th>{{t('total')}}</th><th>{{t('share')}}</th><th>{{t('crit')}}</th></tr></thead><tbody><tr v-for="(p,i) in battle.players" :key="p.id" :class="{chosen:player?.id===p.id}"><td><button class="name" @click="selectedPlayer=p.id"><span class="rank">{{i+1}}</span>{{p.name}}</button></td><td>{{label('classes',p.classId)}}</td><td class="accent">{{fmt(p.dps)}}</td><td>{{fmt(p.damage)}}</td><td><div class="share"><span :style="{width:p.share*100+'%'}"></span><b>{{pct(p.share)}}</b></div></td><td>{{pct(p.critRate)}}</td></tr></tbody></table></div><p class="hint">{{t('selected')}}</p></section>
    <section v-if="player" class="panel"><h2>{{player.name}} <span>／ {{t('skill')}}{{t('battleDamage')}}</span></h2><div class="table-wrap"><table><thead><tr><th>{{t('skill')}}</th><th>{{t('total')}}</th><th>DPS</th><th>{{t('hits')}}</th><th>{{t('crit')}}</th></tr></thead><tbody><tr v-for="s in player.skills" :key="s.id"><td>{{label('skills',s.id)}}<small class="id">{{s.id}}</small></td><td>{{fmt(s.damage)}}</td><td>{{fmt(s.dps)}}</td><td>{{s.hits}}</td><td>{{pct(s.hits?s.crits/s.hits:0)}}</td></tr></tbody></table></div></section>
    <section class="panel"><h2>{{t('timeline')}}</h2><svg viewBox="0 0 1000 150" role="img" :aria-label="t('timeline')"><line x1="10" y1="130" x2="990" y2="130" stroke="#39434f"/><polyline :points="points" fill="none" stroke="#71d7b4" stroke-width="3"/></svg><p class="hint">MOGUGIの中央寄せ5秒窓集計を再利用。縦軸は窓内ダメージ、横軸は戦闘開始からの時間です。</p></section>
   </template><div v-else class="empty">この再生時刻にはパーティの戦闘がありません。</div>
  </template>
  <div v-else class="empty"><span class="empty-mark">M</span><h2>{{t('empty')}}</h2><p>{{t('sampleHint')}}</p></div>
  <footer>{{t('formula')}}<br>実通信・正式な職業／スキルID・ゲーム上の開始終了判定は未検証です。</footer>
 </main>
</template>
