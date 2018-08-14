import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  classNames: ['json-viewer'],

  json: null,
  jsonStr: computed('json', function() {
    return JSON.stringify(this.get('json'), null, 2);
  }),
});
