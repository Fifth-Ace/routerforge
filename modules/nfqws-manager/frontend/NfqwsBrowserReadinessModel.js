// Exact browser gate readiness: HTTP request arrival is not DOM hydration.
// Fail closed if Svelte never publishes the expected rendered state.
export async function waitForRenderedState(probe,{label='DOM',timeoutMs=8000,intervalMs=50,
  now=()=>Date.now(),pause=ms=>new Promise(resolve=>setTimeout(resolve,ms)),beforeProbe=()=>{}}={}){
  if(typeof probe!=='function'||typeof beforeProbe!=='function'||!(timeoutMs>0)||!(intervalMs>0))
    throw Error('BROWSER_READINESS_INVALID_ARGUMENTS');
  const deadline=now()+timeoutMs;
  while(true){
    beforeProbe();
    if(await probe())return true;
    const remaining=deadline-now();
    if(remaining<=0)throw Error('BROWSER_WAIT_TIMEOUT: '+label);
    await pause(Math.min(intervalMs,remaining));
  }
}
