# Combat preview test checklist

Target: test Keenetic Hopper KN-3811 only.

1. Install Core, then optional packages one by one.
2. Verify `/api/health` and SSE after each install.
3. Verify classic DNS at `/api/plain-dns` while a plain port-53 resolver is
   configured in Keenetic.
4. Verify System CPU values move on both cores.
5. Compare Thermal with `/sys/class/thermal` and `/sys/class/hwmon`.
6. Compare Storage capacity with `df`; verify passive rates change under real I/O.
7. Compare Network interfaces/routes with kernel state.
8. Install Profiling and confirm only loopback `:6061` listens.
9. Remove an optional module and confirm Core plus other modules keep running.
10. Marketplace must detect existing AWG Manager/nfqws2 and must not execute
    any third-party installer.
