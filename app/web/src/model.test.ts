import {describe,it,expect} from 'vitest';
import {csvCell,resultCSV} from './model';
describe('CSV export',()=>{
 it('quotes Japanese names and embedded quotes',()=>{expect(csvCell('アオイ,"仮名"')).toBe('"アオイ,""仮名"""');});
 it('neutralizes spreadsheet formula prefixes',()=>{for(const s of ['=1+1',' +cmd','@SUM(1)','-1'])expect(csvCell(s)).toBe('"\''+s+'"');});
 it('emits UTF-8 BOM and explicit unverified provenance',()=>{expect(resultCSV({version:1,region:'jp',source:'synthetic',untilMs:0,idleMs:5000,battles:[],liveVerified:false})).toContain('\ufeff"データ種別","synthetic","地域","jp","実通信検証","false"');});
});
