import Mixin from '@ember/object/mixin';

export default Mixin.create({
  setupDocumentVisibility: function() {
    this.set('_visibilityHandler', this.get('visibilityHandler').bind(this));
    document.addEventListener('visibilitychange', this.get('_visibilityHandler'));
  }.on('activate'),

  removeDocumentVisibility: function() {
    document.removeEventListener('visibilitychange', this.get('_visibilityHandler'));
  }.on('deactivate'),
});
