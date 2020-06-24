import ApplicationSerializer from './application';

export default class RescheduleEvent extends ApplicationSerializer {
  normalize(typeHash, hash) {
    // Time is in the form of nanoseconds since epoch, but JS dates
    // only understand time to the millisecond precision. So store
    // the time (precise to ms) as a date, and store the remaining ns
    // as a number to deal with when it comes up.
    hash.TimeNanos = hash.RescheduleTime % 1000000;
    hash.Time = Math.floor(hash.RescheduleTime / 1000000);

    hash.PreviousAllocationId = hash.PrevAllocID ? hash.PrevAllocID : null;
    hash.PreviousNodeId = hash.PrevNodeID ? hash.PrevNodeID : null;

    return super.normalize(typeHash, hash);
  }
}
