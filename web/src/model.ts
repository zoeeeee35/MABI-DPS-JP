export type Hit = {id:string;atMs:number;type:string;playerId?:string;skillId?:string;targetId?:string;damage?:number;critical?:boolean};
export type CombatLog = {version:number;region:string;source:string;players:{id:string;name:string;classId:string;party:boolean}[];events:Hit[]};
export type Skill = {id:string;damage:number;hits:number;crits:number;dps:number};
export type Row = {id:string;name:string;classId:string;damage:number;hits:number;crits:number;dps:number;share:number;critRate:number;skills:Skill[]};
export type Battle = {number:number;startMs:number;endMs:number;durationSec:number;active:boolean;reason:string;damage:number;players:Row[];damageEvents:Hit[]};
export type Result = {version:number;region:string;source:string;untilMs:number;idleMs:number;battles:Battle[];liveVerified:false};
export function csvCell(v:unknown):string { let s=String(v); if (/^[\s]*[=+@\-]/.test(s)) s="'"+s; return '"'+s.replaceAll('"','""')+'"'; }
export function resultCSV(r:Result):string {
 const rows:unknown[][]=[['データ種別',r.source,'地域',r.region,'実通信検証',false],['戦闘','開始ms','終了ms','戦闘時間（秒）','プレイヤー','職業ID','総ダメージ','DPS','パーティ内割合','クリティカル率','スキルID','命中数']];
 for (const b of r.battles) for(const p of b.players) {
 rows.push([b.number,b.startMs,b.endMs,b.durationSec,p.name,p.classId,p.damage,p.dps,p.share,p.critRate,'',p.hits]);
 for(const s of p.skills) rows.push([b.number,b.startMs,b.endMs,b.durationSec,p.name,p.classId,s.damage,s.dps,p.damage?s.damage/p.damage:0,s.hits?s.crits/s.hits:0,s.id,s.hits]);
 } return '\ufeff'+rows.map(row=>row.map(csvCell).join(',')).join('\r\n');
}
