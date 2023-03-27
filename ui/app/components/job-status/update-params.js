// @ts-check
import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';

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
  @service notifications;

  /**
   * @type {JobDefinition}
   */
  @tracked rawDefinition = null;

  get updateParamGroups() {
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

  @action onError({ Error }) {
    const error = Error.errors[0].title || 'Error fetching job parameters';
    this.notifications.add({
      title: 'Could not fetch job definition',
      message: error,
      color: 'critical',
    });
  }

  @action async fetchJobDefinition() {
    this.rawDefinition = await this.args.job.fetchRawDefinition();
  }
}
