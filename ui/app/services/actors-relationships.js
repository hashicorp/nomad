import Service from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { schedule } from '@ember/runloop';

export default class ActorRelationshipService extends Service {
  @tracked actors = [];

  get fns() {
    const { registerActor, deregisterActor } = this;
    return { registerActor, deregisterActor };
  }

  get data() {
    const { actors } = this;
    return { actors };
  }

  @action registerActor(actor) {
    schedule('actions', this, () => {
      this.actors = [...this.actors, actor];
    });
  }

  @action deregisterActor(actor) {
    schedule('actions', this, () => {
      this.actors = this.actors.filter((a) => a !== actor);
    });
  }
}
