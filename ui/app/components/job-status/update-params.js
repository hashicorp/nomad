// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

/**
 * @typedef {Object} DefinitionUpdateStrategy
 * @property {boolean} AutoPromote
 * @property {boolean} AutoRevert
 * @property {number} Canary
 * @property {number} MaxParallel
 * @property {string} HealthCheck
 * @property {number} MinHealthyTime
 * @property {number} HealthyDeadline
 * @property {number} ProgressDeadline
 * @property {number} Stagger
 */

/**
 * @typedef {Object} DefinitionTaskGroup
 * @property {string} Name
 * @property {number} Count
 * @property {DefinitionUpdateStrategy} Update
 */

/**
 * @typedef {Object} JobDefinition
 * @property {string} ID
 * @property {DefinitionUpdateStrategy} Update
 * @property {DefinitionTaskGroup[]} TaskGroups
 */

export default class JobStatusUpdateParamsComponent extends Component {
  /**
   * @type {JobDefinition}
   */
  @tracked rawDefinition = null;

  get updateParams() {
    if (this.rawDefinition) {
      return this.rawDefinition.TaskGroups.map((tg) => {
        return {
          name: tg.Name,
          update: tg.Update,
        };
      });
    } else {
      return null;
    }
  }

  @action async fetchJobDefinition() {
    this.rawDefinition = await this.args.job.fetchRawDefinition();
  }
}
