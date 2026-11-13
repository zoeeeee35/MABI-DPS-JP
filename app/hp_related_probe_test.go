package main
import("testing";"os";"encoding/binary";"fmt";"encoding/json")
func TestHPRelatedFieldProbe(t *testing.T){
 path:=os.Getenv("MABI_HP_START");if path==""{t.Skip("local")};values:=map[uint64]map[uint64]bool{}
 f,e:=os.Open(path);if e!=nil{t.Fatal(e)}
 _,e=replayPCAPObserved(f,16000,func(id uint32,p []byte,out bool){if !out&&id==0x1d596&&len(p)==21&&binary.LittleEndian.Uint32(p[8:])==8{entity:=binary.LittleEndian.Uint64(p);if values[entity]==nil{values[entity]=map[uint64]bool{}};values[entity][binary.LittleEndian.Uint64(p[12:])]=true}});f.Close();if e!=nil{t.Fatal(e)}
 hits:=map[string]int{};sameEntity:=map[string]int{};f,e=os.Open(path);if e!=nil{t.Fatal(e)};defer f.Close()
 _,e=replayPCAPObserved(f,16000,func(id uint32,p []byte,out bool){if out||id==0x1d596||len(p)<12||len(p)>4096{return};entity:=binary.LittleEndian.Uint64(p);known:=values[entity];if known==nil{return};sameEntity[fmt.Sprintf("%X",id)]++;for off:=8;off+4<=len(p);off++{n:=uint64(binary.LittleEndian.Uint32(p[off:]));if n>0&&known[n]{hits[fmt.Sprintf("id=%X/entity=%X/length=%d/offset=%d/value=%d",id,entity,len(p),off,n)]++}}});if e!=nil{t.Fatal(e)}
 b,_:=json.MarshalIndent(map[string]any{"sameLeadingEntity":sameEntity,"coincidencesNotConfirmedHP":hits},"","  ");if p:=os.Getenv("MABI_HP_PROBE_REPORT");p!=""{if e=os.WriteFile(p,b,0600);e!=nil{t.Fatal(e)}}
}
