import Component from '@ember/component';
import { computed } from '@ember/object';

export default Component.extend({
  job: null,
  classNames: ['boxed-section'],

  isExpanded: computed(function() {
    const storageValue = window.localStorage.nomadExpandJobSummary;
    return storageValue != null ? JSON.parse(storageValue) : true;
  }),

  persist(item, isOpen) {
    window.localStorage.nomadExpandJobSummary = isOpen;
    this.notifyPropertyChange('isExpanded');
  },
});
