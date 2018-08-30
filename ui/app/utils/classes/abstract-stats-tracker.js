import Mixin from '@ember/object/mixin';
import { assert } from '@ember/debug';

export default Mixin.create({
  url: '',

  fetch() {
    assert('StatTrackers need a fetch method, which should have an interface like window.fetch');
  },

  append(/* frame */) {
    assert(
      'StatTrackers need an append method, which takes the JSON response from a request to url as an argument'
    );
  },

  poll() {
    const url = this.get('url');
    assert('Url must be defined', url);

    return this.get('fetch')(url)
      .then(res => {
        return res.json();
      })
      .then(frame => this.append(frame));
  },
});
