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

  // Uses EC as a form of debounce to prevent multiple
  // references to the same tracker from flooding the tracker,
  // but also avoiding the issue where different places where the
  // same tracker is used needs to coordinate.
  poll: task(function*() {
    const url = this.get('url');
    assert('Url must be defined', url);

    yield this.get('fetch')(url)
      .then(res => res.json())
      .then(frame => this.append(frame));

    yield timeout(2000);
  }).drop(),
});
