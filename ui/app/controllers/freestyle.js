import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import FreestyleController from 'ember-freestyle/controllers/freestyle';

export default FreestyleController.extend({
  emberFreestyle: service(),

  timerTicks: 0,

  startTimer: function() {
    this.set(
      'timer',
      setInterval(() => {
        this.incrementProperty('timerTicks');
      }, 500)
    );
  }.on('init'),

  stopTimer: function() {
    clearInterval(this.get('timer'));
  }.on('willDestroy'),

  distributionBarData: computed(() => {
    return [
      { label: 'one', value: 10 },
      { label: 'two', value: 20 },
      { label: 'three', value: 30 },
    ];
  }),

  distributionBarDataWithClasses: computed(() => {
    return [
      { label: 'Queued', value: 10, className: 'queued' },
      { label: 'Complete', value: 20, className: 'complete' },
      { label: 'Failed', value: 30, className: 'failed' },
    ];
  }),

  distributionBarDataRotating: computed('timerTicks', () => {
    return [
      { label: 'one', value: Math.round(Math.random() * 50) },
      { label: 'two', value: Math.round(Math.random() * 50) },
      { label: 'three', value: Math.round(Math.random() * 50) },
    ];
  }),
});
