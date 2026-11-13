// Exact read-only test contract for /v1/v2/strategies (Go: strategy_library_v2.go).
// No persistent writes, no router data, and no fake mutation success.
export const STRATEGY_LIBRARY_FIXTURE = Object.freeze({
  ok:true, version:1, count:0, max:64, strategies:Object.freeze([])
});
export function strategyFixtureReply(method,path,body=''){
  if(!['/v2/strategies','/v2/strategies/save','/v2/strategies/delete'].includes(path))return null;
  if(method==='GET'&&path==='/v2/strategies')return {status:200,payload:STRATEGY_LIBRARY_FIXTURE};
  if(method==='POST'&&['/v2/strategies/save','/v2/strategies/delete'].includes(path)){
    let confirm='';
    try{confirm=JSON.parse(body)?.confirm}catch{}
    const required=path==='/v2/strategies/save'?'ROUTERFORGE_V2_STRATEGY_SAVE':'ROUTERFORGE_V2_STRATEGY_DELETE';
    return {status:403,payload:{error:confirm===required?'TEST_ONLY_NO_PERSISTENCE':'CONFIRM_REQUIRED'}};
  }
  return {status:405,payload:{error:'TEST_ONLY_METHOD_NOT_ALLOWED'}};
}
