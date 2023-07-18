## Today's goals:
- talk through passing data from an endpoint through to the user
- dive a little deeper into Ember models, adapters, and serializers

---

## Let's expose a property we dont otherwise care about in the UI
  - If the API endpoint gives us roughly what we want to show the user, it's pretty simple!
  - We only need to note it as an `@attr()` in the model of what's being fetched. If its type and format match what we want to show the user (a string rather than a timestamp, etc.), we're good.
  - Once the @attr is added, we can user general Ember conventions on it, like call object.property and watch for changes on it.
  - Let's try exposing Evaluation.SnapshotIndex on the evaluations page; pop open ui/app/templates/evaluations/index.hbs
  - Quick debugging note: You can write `{{log "whatever you want"}}` to generate output to the browser console from a template. Let's add `{{log row.model row.model.snapshotIndex}}` to line ~86.
  - Notice how our snapshotIndex is undefined? Let's pop open ui/app/models/evaluation.js and add `@attr('number') snapshotIndex;`
  - Refresh our page again, we have numbers!
  - Adding it to the page: a whole lot of what we show in the Nomad UI is exposed in tables. 

---

## When what we want to show the user isn't the same as our endpoint provides

- We do this pretty often, like when our modifyTime come back as nanosecond timestamps and we want them as a "X days ago" string (deployment.job.submitTime), or an "in 10 minutes" (token.TTL); or maybe something makes sense as a K/V dictionary in our endpoint but makes more sense to display as an array in a form in the browser (Variables)
- There are 3 good places to do this, ordered from most-upstream to most-downstream:
  1. At the Serializer layer. the `normalize` function gets called before Ember knows anything about the object that just got returned from our endpoint. We can modify the raw hash here.
  2. At the Model layer, we can create a `get()` method that modifies something as we see fit.
  3. At the component layer, we can create an isolated change that only matters in the context of the page we're on.

Let's explore how and why we'd do all 3, in reverse.

---

## Changing a property in an isolated, specific context

### Why here?
- If we are generally happy with the behavio(u)r of our new property throughout the app, but in one context it ought to be changed, we can do it in our component or template. Changes here would be contextual

### What's it look like?
- Let's add some functions to our component and check against them on the fly in ui/app/controllers/evaluations/index.js:

```
  @action hasEvenSnapshotIndex(index) {
    return !!(index % 2);
  }

  @action isHighestOnPage(index) {
    return this.model.mapBy('snapshotIndex').sort((a,b) => b - a).indexOf(index) === 0;
  }
```

and in our template, ui/app/templates/evaluations/index.hbs:

```
{{compute (action this.isHighestOnPage) row.model.snapshotIndex}}
```


## Creating a generally-available computed property in a model

### Why here?
- If we think we probably want to retain our original property in some contexts, but create a modifier that's used in others, the model is a good place to do it. At a place further-along than the serializer, it can also see things like other, unrelated models. A good place we use this is [Path-linked variables](https://github.com/hashicorp/nomad/blob/main/ui/app/models/job.js#L358-L368) for jobs — we can know, from a job page, whether variables exist that match that job's naming convention. 

### What's it look like?
- Let's pop open ui/app/models/evaluation.js and make a new getter:

```
  get snapshotIndexIsEven() {
    return this.snapshotIndex % 2 === 0;
  }
```
- From our template, we can call this computed property / getter just like our @attr() from before:

```
{{row.model.snapshotIndexIsEven}}
```

## Modifying the payload, as far as our app is concerned

### Why here?
- If we know that the only context in which something is useful is in a modified state, we can fully manipulate it at the serializer layer. We can rename the property itself, change its format, change its meaning, etc. All instances of the thing we're modifying in the front-end will adhere to the rules we set here.

### What's it look like?
- Let's say the thing we really cared about isn't the snapshotIndex, it's the snapshotCreateDelta.
- Open up ui/app/serializers/evaluation.js and check out the normalize() method. the `hash` object is, capitalization and all, what comes back from our API. We can modify things here by doing something like this:

```
hash.SnapshotCreateDelta = hash.SnapshotIndex - hash.CreateIndex;
delete hash.SnapshotIndex;
```

and in our model,
```
@attr('number') snapshotCreateDelta;
```

and finally, in our template,
```
{{row.model.snapshotCreateDelta}}
```

And we can do all sorts of fun downstream stuff now that we've made a change at this high level; try opening up ui/app/templates/jobs/job/evaluations.hbs and add a new sortable header:

```
<t.sort-by @prop="snapshotCreateDelta">DELTA</t.sort-by>
```

and a cell for it:

```
<td>{{row.model.snapshotCreateDelta}}</td>
```