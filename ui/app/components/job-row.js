import { inject as service } from '@ember/service';
import Component from '@ember/component';
import { lazyClick } from '../helpers/lazy-click';

export default Component.extend({
  store: service(),

  tagName: 'tr',
  classNames: ['job-row', 'is-interactive'],

  job: null,

  onClick() {},

  click(event) {
    lazyClick([this.get('onClick'), event]);
  },
});
