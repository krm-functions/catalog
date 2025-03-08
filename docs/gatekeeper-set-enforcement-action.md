# Set GateKeeper constraint enforcement actions

This function will set the `enforcementAction` field on GateKeeper constraints.

## Example

```shell
grep enforcementAction examples/gatekeeper-set-enforcement-action/*
  enforcementAction: dryrun

kpt fn source examples/gatekeeper-set-enforcement-action | \
    kpt fn eval - -i ghcr.io/krm-functions/gatekeeper-set-enforcement-action -o unwrap -- enforcementAction=deny > out.yaml

grep enforcementAction out.yaml
```
