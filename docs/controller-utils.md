# Controller Utils

A framework for running kubernetes client commands and performing actions related to reconciling.

## Controller Client

The controller client is a non-thread safe tool that is initialized to provide the context to the actions

## What's an action?

Actions are structs that have two functions associated to them: Bind and Exec. They are meant to encode logic into a call. For instance, the CreateAction will attempt to create a kubernetes runtime object. But there are options that can be passed in to extend the basic create with additional functionality. Like using an annotator or add the controller as an owner.

The Bind function allows the action to consider the result from a previous action. This can be useful for chaining actions together in one executable flow.

The Exec funtion performs the action. It requires a context object and the ClientCommand runner.

The big advantage of a predefined action is the reuse of code and reduction of boiler plate. Additionally, since actions are interfaces, anyone can extend it.

## Action States

Actions have a few states:

| State    | Description                                                                             | Returned By                |
| :------- | :-------------------------------------------------------------------------------------- | :------------------------- |
| Continue | Instructs the reconciler to keep going                                                  | All                        |
| NotFound | Instructs the reconciler the last command returned a NotFound error                     | GetAction                  |
| Requeue  | Instructs the reconciler that an action was performed and the result should be requeued | CreateAction, UpdateAction |
| Error    | Instructs the reconciler the last command returned an error                             | All                        |

## Actions

### Do Action

The Do action is a higher order action that wraps other actions. Specifically it takes a list of actions that will be performed sequentially and each result will be bound to the next action.

### List command

### Additional Actions

- Create
