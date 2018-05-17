import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';

export default Component.extend({
  store: service(),

  job: null,

  summary: alias('job.summary'),

  classNames: ['boxed-section'],
});
