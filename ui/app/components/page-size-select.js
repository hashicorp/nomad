import Component from '@ember/component';
import { inject as service } from '@ember/service';

export default Component.extend({
  userSettings: service(),

  tagName: '',
  pageSizeOptions: Object.freeze([10, 25, 50]),

  onChange() {},
});
