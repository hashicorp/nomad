import Component from '@ember/component';
import { inject as service } from '@ember/service';

export default Component.extend({
  tagName: '',

  userSettings: service(),

  pageSizeOptions: Object.freeze([10, 25, 50]),
});
