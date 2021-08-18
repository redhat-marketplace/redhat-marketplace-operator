00 Enabled userWorkloadMonitoring on cluster & spec, transitional state, verify MeterDefinition has results from new uwm prometheus provider
20 Enable userWorkloadMonitoring on cluster & spec in MeterBase before MarketplaceConfig up, such that transitional state is skipped, verify MeterDefinition has results from uwm prometheus provider
30 From 20, disable userWorkloadMonitoring on spec, transitional state, verify MeterDefinition has results from rhm prometheus provider
40 Enable userWorkloadMonitoring on only spec, check condition reason why it's not enabled on cluster
