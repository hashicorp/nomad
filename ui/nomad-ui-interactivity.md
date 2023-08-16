## Today's goals:
- learn how to tie HTML/Handlebars elements to JS functions
- get comfortable with a few Helios components as a shortcut
- talk about the "Data Down, Actions Up" pattern

---

## Review: Handling interaction in plain Javascript
  - Javascript has an `.addEventListener()` method that takes a string (name of the event to watch for, like "click") and a function (what you want to happen)
  - Events that call the function will get an inherent parameter: the JS click/tap/etc. event itself.
  - Easier to show: Pop open the console on absolutely any webpage you want and drop this in:

  ```
  let myInteraction = (ev) => { console.log('howdy', ev) };
  document.addEventListener('click', myInteraction)
  ```

  - And because this'll get annoying quickly, here's how to remove the handler:

  ```
  document.removeEventListener('click', myInteraction);
  ```
---
## plain JS pt 2
  - Click around your page a bit; check out the `PointerEvent` that comes back. Some notable properties to highlight (right click + "Save object as global variable" for ease of temp1.propName use)
    - `ev.target` to get the element you clicked on. Try `ev.target.innerText`
    - `ev.clientX` and `ev.clientY` to get the x/y position of your mouse on the screen 
  - "click" isn't the only event you can bind to; try "mousemove" like this:

  ```
  let findMouse = (ev) => { console.log(ev.clientX, ev.clientY) };
  document.addEventListener('mousemove', findMouse)
  ```
---
## plain JS pt 3
  - There are a LOT of different types of events; See [MDN's Event Reference Page](https://developer.mozilla.org/en-US/docs/Web/API/Document#events) for more, or for different properties of events, check out for example [MDN's Pointer Event Properties](https://developer.mozilla.org/en-US/docs/Web/API/PointerEvent#instance_properties)

  - Similarly, you can swap `document` for `document.getElementsByTagName('h1')[0]` etc. to add interaction to a specific thing.

---
## Handling interaction in Ember
  - All these JS event listeners can get out of hand quickly, and they're both defined and attached to an element in the logic layer. A general preference is to handle event/element attachments in the template layer, instead.
  - This is done using Ember's `{{on}}` modifier. For example, `<button {{on "click" this.someFunc}}` is a typical pattern.
    - Note: you'll also sometimes see `<button @onclick={{action "somefunc"}}` in our codebase but this is a deprecated style.

---
## Ember pt 2
  - Let's take a look at some examples: pull open a client page in Nomad and then open `ui/app/components/metadata-kv.hbs`
    - Check out the `<button>` on line 15:
    ```
    {{on "click" (queue
      (action @onKVSave (hash key=this.prefixedKey value=this.value))
      (action (mut this.editing) false)
    )}}
    ```
    - We are queueing events here using the [Queue helper](https://github.com/DockYard/ember-composable-helpers);
    - The first one is inheriting a method called `onKVSave` from a parent route/component. This is a common pattern for component re-usability; letting a parent decide what the child actions shoudl trigger.
    - The second one, `(mut)`, is a shorthand for setting a property in our component without a method. We easily could have made a function to do this in our component JS, but this is leaner.

---
## Ember pt 3
  - Let's add a third handle here: `(action this.myFunc 123)`;
  - open `ui/app/components/metadata-kv.js` and let's add a simple method here:

  ```
    @action myFunc(num, event) {
      console.log('myFunc called with', num, event);
    }
  ```

  - Just like plain JS functions, the `event` is the implicitly-passed; but it's always the last param.
  - Using the `(action)` / `@action` synctax has some nuance to it, but the gist is:
    - it lets you pass in arbitrary args to the method, like 123
    - it lets use you the `this` keyword in the context of your logic

---
## Ember pt 4
  - Let's look at another kind of event. Open `ui/app/components/metadata-editor.hbs` and check out the `<Input>` element. (very small sidenote: capital I on Input means that this is an Ember wrapper around the native html `<input>` element. Adds some syntactic sugar.)
  - This element has a different event: `keyup`, and a different method signature: `@onEdit` instead of `this.onEdit`. If we recall from a few sessions ago, the `@` declaration on a component property like this means it's passed down from a parent. In fact, we'll find `onEdit` in the `metadata-kv.js` file we had open a moment ago. Let's edit that with something like:
  ```
  if (event.target.value === "magic") {
    alert('You found the magic word!');
  }
  ```

---
## Ember pt 5
  - We could even make this outside-of-context aware if we wanted to:

  ```
  {{on "keyup" (action @onEdit @kv.key)}}
  ```
  ```
  @action onEdit(peerKey, event) {
    if (event.key === 'Escape') {
      this.editing = false;
    }
    if (event.target.value === peerKey) {
      alert('cant match key and value')
    }
  }
  ```

---
## Some notes on reusability
  - The metadata-editor.hbs file is a good example of what Ember calls "Data Down, Actions Up" (DDAU)
  - It inherits its relevant data from its parent, `<MetadataKV>`, which passes them like this:
  ```
  <MetadataEditor
    @kv={{hash key=this.prefixedKey value=this.value}}
    @onEdit={{this.onEdit}}
  ```
  - This means that the MetadataEditor component doesn't have to know about how to get its key/value (via fetch or something), nor what happens after it clicks them (it just passes the action up to @onEdit)
  - We can see the benefit of this in clients/client/index.hbs, which uses the `<MetadataEditor>` component in a different context: adding NEW metadata:
  ```
  <MetadataEditor
    @kv={{this.newMetaData}}
    @onEdit={{this.validateMetadata}}
  >
  ```
  - This approach promotes clear data flow and makes components more predictable and easier to test.

---
## Helios
  - Hashicorp has our very own library of Ember-friendly common components called Helios (https://helios.hashicorp.design/components)
  - Several of these are interaction-ready and will handle things like hover states, accessibility, etc. very well under the hood
  - Take for example the `<Hds::Alert>` component: it comes with buttons that are `{{on "click"}}`-ready: https://helios.hashicorp.design/components/alert?tab=code#actions-1

---
## ~Fin~

