import Ember from 'ember';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { fragment } from 'ember-data-model-fragments/attributes';

const { computed } = Ember;

export default Model.extend({
  // Available from list response
  name: attr('string'),
  datacenter: attr('string'),
  isDraining: attr('boolean'),
  status: attr('string'),
  statusDescription: attr('string'),

  // Available from single response
  httpAddr: attr('string'),
  tlsEnabled: attr('boolean'),
  attributes: fragment('node-attributes'),
  resources: fragment('resources'),
  reserved: fragment('resources'),

  address: computed('httpAddr', function() {
    const addr = this.get('httpAddr');
    return addr && addr.split(':')[0];
  }),

  port: computed('httpAddr', function() {
    const addr = this.get('httpAddr');
    return addr && addr.split(':')[1];
  }),
});
