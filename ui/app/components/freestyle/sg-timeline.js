import Component from '@ember/component';
import { computed } from '@ember/object';
import moment from 'moment';

export default Component.extend({
  yesterday: computed(() => moment().subtract(1, 'd')),
  today: computed(() => moment()),
});
