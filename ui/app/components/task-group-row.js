import Component from '@ember/component';
import { lazyClick } from '../helpers/lazy-click';

export default Component.extend({
  tagName: 'tr',

  classNames: ['task-group-row', 'is-interactive'],

  taskGroup: null,

  onClick() {},

  click(event) {
    lazyClick([this.get('onClick'), event]);
  },
});
