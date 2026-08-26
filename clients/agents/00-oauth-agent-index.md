# RVPay Clients OAuth Agent Bundle

Run agents in this order:

1. `01-oauth-context-sweep.md`
2. `02-oauth-marketplace-fix.md`

Agent 01 is reconnaissance and creates the clients-service context/control files.
Agent 02 uses those files to make the narrowly scoped implementation and test changes.
