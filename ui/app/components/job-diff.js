import Ember from 'ember';

const { Component, computed } = Ember;

export default Component.extend({
  classNames: ['job-diff'],
  classNameBindings: ['isEdited:is-edited', 'isAdded:is-added', 'isDeleted:is-deleted'],

  diff: null,

  verbose: true,

  isEdited: computed.equal('diff.Type', 'Edited'),
  isAdded: computed.equal('diff.Type', 'Added'),
  isDeleted: computed.equal('diff.Type', 'Deleted'),
});
