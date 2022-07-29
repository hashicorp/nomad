import Model from '@ember-data/model';
import { attr } from '@ember-data/model';

export default class Policy extends Model {
  @attr('string') name;
  @attr('string') description;
  @attr('string') rules;
  @attr() rulesJSON;

  // Gets the name of the policy, and if it starts with _:/, delimits on / and returns each part separately in an array

  /**
   * @typedef nameLinkedEntities
   * @type {Object}
   * @property {string} namespace
   * @property {string} [job]
   * @property {string} [group]
   * @property {string} [task]
   */

  /**
   * @type {nameLinkedEntities}
   */
  get nameLinkedEntities() {
    const entityTypes = ['namespace', 'job', 'group', 'task'];
    const emptyEntities = { namespace: '', job: '', group: '', task: '' };
    if (this.name?.startsWith('_:/') && this.name?.split('/').length <= 5) {
      return this.name
        .split('/')
        .slice(1, 5)
        .reduce((acc, namePart, index) => {
          acc[entityTypes[index]] = namePart;
          return acc;
        }, emptyEntities);
    } else {
      return emptyEntities;
    }
  }
}
