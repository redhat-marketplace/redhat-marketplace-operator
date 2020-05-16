# Controller testing

To ease testing, there is a mini-framework available in the test/controller package.
This framework provides 3 primitives for testing controllers although more can easily be added.

The 4 primitives is list, get, and reconcile. Using the framework allows you to define tests that are easy to reason with.

The problem with writing tests for the reconciler is you lose track of where exactly you are and why you're their. The simple flow is:

1. look for something (configmap, pod, deployment)
1. if not found create that missing thing and requeue
1. if found, update it to match the desired state
1. update status

These 4 steps can be performed over and over. So knowing exactly where you are and why you're there can be hard sometimes. The higher up you can see the problem the easier its is.

## Example
