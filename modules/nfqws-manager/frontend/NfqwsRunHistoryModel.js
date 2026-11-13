/** Read-only scoring and guard thresholds. No network or persistent state. */
export function measuredAutoPickEligible(row){
  return Number(row?.runs)>=2 && Number(row?.targets)>=3 &&
    Number(row?.reach)>=.75 && Number(row?.stable)>=.60 &&
    Number.isFinite(Number(row?.median_score));
}
export function guardThresholdFraction(value){
  const n=Number(value);
  if(![15,20,25,30].includes(n))throw Error('NFQWS_RECOVERY_THRESHOLD_UNSUPPORTED');
  return n/100;
}
export function recoveryRegression(before,after,thresholdPercent){
  const threshold=guardThresholdFraction(thresholdPercent);
  if(!before||!after||![before.reach,before.stable,after.reach,after.stable].every(x=>typeof x==='number'&&Number.isFinite(x)&&x>=0&&x<=1))
    throw Error('NFQWS_RECOVERY_METRICS_INVALID');
  const reachDrop=before.reach-after.reach,stableDrop=before.stable-after.stable;
  return {regressed:Math.round(reachDrop*1e9)>=Math.round(threshold*1e9)||Math.round(stableDrop*1e9)>=Math.round(threshold*1e9),reachDrop,stableDrop,threshold};
}
