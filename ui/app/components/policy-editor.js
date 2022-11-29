import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';

export default class PolicyEditorComponent extends Component {
  @service flashMessages;
  @service router;
  @service store;

  @alias('args.policy') policy;

  @action async save(e) {
    e.preventDefault();
    try {
      const nameRegex = '^[a-zA-Z0-9-]{1,128}$';
      if (!this.policy.name?.match(nameRegex)) {
        throw new Error(
          'Policy name must be 1-128 characters long and can only contain letters, numbers, and dashes.'
        );
      }

      if (
        this.policy.isNew &&
        this.store.peekRecord('policy', this.policy.name)
      ) {
        throw new Error(
          `A policy with name ${this.policy.name} already exists.`
        );
      }

      this.policy.id = this.policy.name;

      await this.policy.save();

      this.flashMessages.add({
        title: 'Policy Created!',
        type: 'success',
        destroyOnClick: false,
        timeout: 5000,
      });

      this.router.transitionTo('policies');
    } catch (error) {
      this.flashMessages.add({
        title: `Error creating Policy ${this.policy.name}`,
        message: error,
        type: 'error',
        destroyOnClick: false,
        sticky: true,
      });
      throw error;
    }
  }
}
