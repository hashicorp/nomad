## Today's goals:
- get you comfortable with a couple arch concepts of Nomad's UI
- get you comfortable with running some of the most common Ember CLI commands
- go over typical data flow from request to render in Nomad

## Today's non-goals:
- JS, Ember, and Handlebars syntax how-to
- talk about peripheral tools like Mirage, Percy, and Storybook
- Styles/CSS/HashiCorp Helios

---

## Ember at a high-level
  - Nomad's UI is written in a JavaScript framework called Ember. It's comparable to React and other frameworks. 2 ways it differentiates itself worth mentioning:
    - Very opinionated about the "right" way to do things. (Mantra heavy; "Data Down, Actions Up"; Smalltalk-like MVC; "Templates dont have logic".)
    - Tons of built-in data/relationship modelling (EmberData)

A lot of what I'm going to go over are Ember conventions more than they are JS or Nomad conventions.

---

## JS Conventions that we use:
- a package.json file in the /ui route that specifies our dependencies and common scripts
  - Small sampling of package.json/scripts:
```
"scripts": {
  "build": "ember build --environment=production",
  "start": "ember server",
  "test": "npm-run-all lint test:*",
  "lint:js": "eslint . --cache",
  "exam": "percy exec -- ember exam --split=4 --parallel",
  "local:qunitdom": "ember test --server --query=dockcontainer",
}
```

- Yarn instead of npm
  - There were great reasons for yarn over npm awhile ago (lockfiles and workspaces were the big ones) but npm has mostly caught up on the big ones. At this point it's just one of our many conventions. Today the big bonus is you don't have to type the word "run" 100 times a day.
  - you're generally running `yarn start` instead of `npm run start`, etc.
- QUnit (run with `ember test`) for testing
  - In practice, I run `yarn local:qunitdom` all day every day
- Dynamic and optional typing (I like [JSDoc](https://jsdoc.app/) because it doesnt intersect with the logic, your comments live beside it. See an [example from our UI](https://github.com/hashicorp/nomad/blob/ede662a828e11b13a2962b85481a0b60046e8121/ui/app/models/variable.js#L15-L50))
- Compilation via build to browser-interpretable JS (stored in ui/dist)

---

## Running the Nomad UI locally
Do this stuff once:
- Install the Ember CLI globally with `npm install -g ember-cli`
- Install yarn globally with `npm install -g yarn` (lol)

Do this to run the ui at localhost:4200:
- **To run with mocked data**: `yarn start`
- **To run against your running nomad agent**: `USE_MIRAGE=false yarn start`

---

## Making new things in the Nomad UI with `ember generate`

If you are making something brand new in the UI, and if the user is going to see it, it probably fits in one of two categories:
- A "thing" that goes someplace that already exists (a bar chart, a list of links, a footer, a copy-to-clipboard button) in Ember is a *component*.
- A "page" that will be linked to (a `route`, a `controller`, and a `template`)

- Ember CLI comes with the concept of "blueprints"; you can use `ember g <component|template|route|controller|serializer|model|acceptance-test>` to get something like this:

```
ember g component donut-chart
installing component
  create app/components/donut-chart.hbs
  skip app/components/donut-chart.js
  tip to add a class, run `ember generate component-class donut-chart`
installing component-test
  create tests/integration/components/donut-chart-test.js
```

^--- note the tip! It skips adding backing logic (.js) and just creates the template (.hbs); let's go open it up:

---

## Editing a component

donut-chart.hbs will look like this:

```
{{yield}}
```

which:
- you can ignore for now
- you know what, actually, you can delete it for now
- please do not push a donut chart to main
- please!

Try replacing it with `<h1>Hello Pals</h1>`

There's not much to be seen until you drop the component in someplace. Open `app/templates/jobs/index.hbs` and invoke our new component with `<DonutChart />`.

It looks just like a standard `<html>` tag, except it's PascalCased.

Passing data into the component is most easily done by using @attrs, like this:

`<DonutChart @yall="buds" />`

Then back in our donut chart, we can replace "Pals" with `@yall`.

For a sensible default, you can even use some Ember template helpers like this:

`<h1>Hello {{or @yall "nerds"}}</h1>`

---

## Backing logic for a component

The part that was skipped during our `ember g` was the component class. Naming conventions within Ember tie them together. Running `ember g component-class donut-chart` will give us a new file at `app/components/donut-chart.js`.

Let's look at how we can separate concerns within our components by using a component class. Open the new file and add a new ***getter***

```
import Component from '@glimmer/component';

export default class DonutChartComponent extends Component {
  get angry() {
    return this.args.yall.toUpperCase();
  }
}
```

and in donut-chart.hbs:
```
<h1>What is up {{this.angry}}</h1>
```

A few notes:
- when you're referring to a passed in variable, you use `@varName` in your template, or `this.args.varName` in your component class.
- when you're referring to a self-contained variable, you use `this.varName` in both template and component class.
- the getter we just wrote is reactive — it knows what it needs, in order to generate its end state, and if those things update? It also updates. (There is some nuance here, see [@tracked](https://api.emberjs.com/ember/5.0/functions/@glimmer%2Ftracking/tracked) for example)

### Brief aside: adding interaction
- Our new component class can also have functions that can be called circumstantially. Try adding this:

```
  sayHi() {
    console.log('hi');
  }
```
and in our component template:

```
<h1 {{on "click" this.sayHi}}>What is up {{this.angry}}</h1>
```

- "click" can easily be "mouseenter", etc.
- Not worth getting into now, but you will often see the UI using keywords like `action` or `perform` in event handlers, for passing params and doing async operations, respectively.
- You can pass functions down into a component just like you would an attribute. `{{on "click" @somePassedFunc}}` is a common convention.

---

## Data Flow: Models, Serializers, Adapters, and the EmberData Store

- [EmberData](https://github.com/emberjs/data) handles data flow from our API into our various routes, components, and services in the Nomad UI. Some of its conventions will feel familiar if you're used to other MVC-looking frameworks, but we'll go over them anyway. (All of the things we're about to talk about can be generated with `ember g <model|serializer|adapter>` etc.)
- **Models** define the properties, behaviors, and relationships of the kinds of data objects we use in the app. Take a look at the [Service Model](https://github.com/hashicorp/nomad/tree/main/ui/app/models/service.js) for an example.
  - Note the `@belongsTo` / `@hasMany` that you'll see in our models. These let us, when interacting with an instance of a model, do things like `this.particularService.node.name` and have things just work. Or `this.someDeployment.allocations.mapBy('clientStatus')`.
  - Functional purity matters more in a model's getter than it might in a component's getter, etc. because of likelihood of high-cost operations when many data objects are present or being considered. Ex: stuff like `this.jobs.map(j => j.services.map(s => s.derivedLevel))` is not exactly common, but not exactly rare either.
- **Adapters** are in charge of how we direct traffic to our API endpoints. See the [Auth Method Adapter](https://github.com/hashicorp/nomad/tree/main/ui/app/adapters/auth-method.js) for how we might use this.
  - Some straightforward things here, like "auth method endpoints should be prefixed with /acl", or "it should be /acl/auth-methods, not /acl/authmethods"
  - Some more custom ones, like "there should be a thing called `someJob.plan()` that `POST`s to a specific endpoint with the following passed data"
- **Serializers** let us convert/handshake data between our endpoints and our UI components. For example, in our /agent/members endpoint, the `Members` list property will be null when empty, and it's much easier to just [handshake that to be an empty array](https://github.com/hashicorp/nomad/tree/main/ui/app/serializers/agent.js#L38-L45) for JS operations' sake.
  - This also works with outgoing data. For example, our `variables` endpoints like having key/values as a dictionary, but the UI like to iterate over all keys/values so treats them like an array. When the user edits that array, [we convert them back to an object upon save](https://github.com/hashicorp/nomad/blob/main/ui/app/serializers/variable.js#L52-L55).
- The **Store** is an Ember Service (sort of an app-wide state convention) that lets us look up certain things from arbitrary places in the app. For example, our donut chart might want to know about all the jobs we currently have. We might add something like this:

```
import { inject as service } from '@ember/service';
...
@service store;

get allCurrentlyLoadedJobs() {
  return this.store.peekAll('job');
}

get aSpecificJobFromServer() {
  return this.store.findRecord('job', 'some-job-id');
}
```

In practice, we try to keep data fetching to top-level routes much more than reusable components, then filter and pass that data directly into the sub-components that need it. It's not unheard of to do reach out to endpoints from arbitrary spots (adding an example token to a policy, for example), but this is for special cases.