import Ember from 'ember';

const { run, $ } = Ember;

export default Ember.Mixin.create({
  setupWindowResize: function() {
    run.scheduleOnce('afterRender', this, () => {
      this.set('_windowResizeHandler', this.get('windowResizeHandler').bind(this));
      $(window).on('resize', this.get('_windowResizeHandler'));
    });
  }.on('didInsertElement'),

  removeWindowResize: function() {
    $(window).off('resize', this.get('_windowResizeHandler'));
  }.on('willDestroyElement'),
});
