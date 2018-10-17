import Mixin from '@ember/object/mixin';
import { assert } from '@ember/debug';
import { task, timeout } from 'ember-concurrency';

export default Mixin.create({
  url: '',

  bufferSize: 500,

  fetch() {
    assert('StatsTrackers need a fetch method, which should have an interface like window.fetch');
  },

  append(/* frame */) {
    assert(
      'StatsTrackers need an append method, which takes the JSON response from a request to url as an argument'
    );
  },

  pause() {
    assert(
      'StatsTrackers need a pause method, which takes no arguments but adds a frame of data at the current timestamp with null as the value'
    );
  },

  // Uses EC as a form of debounce to prevent multiple
  // references to the same tracker from flooding the tracker,
  // but also avoiding the issue where different places where the
  // same tracker is used needs to coordinate.
  poll: task(function*() {
    // Interrupt any pause attempt
    this.get('signalPause').cancelAll();

    try {
      const url = this.get('url');
      assert('Url must be defined', url);

      yield this.get('fetch')(url)
        .then(res => res.json())
        .then(frame => this.append(frame));
    } catch (error) {
      throw new Error(error);
    }

    yield timeout(2000);
  }).drop(),

  signalPause: task(function*() {
    // wait 2 seconds
    yield timeout(2000);
    // if no poll called in 2 seconds, pause
    this.pause();
  }).drop(),
});
