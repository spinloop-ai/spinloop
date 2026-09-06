/**
 * Finding seed instances by identity.
 *
 * The seed Lambda uses this for its join, list and stop paths, and the start
 * Lambda's weights gate uses it to find the seed for the weights an
 * environment would sync — one filter, so the two Lambdas agree on which
 * instance is a seed's compute.
 *
 * The results still carry stopped instances: the caller decides what "in
 * flight" means, and the seed's joined-state logic and the start's gate count
 * only pending and running, the same way.
 */

import { findManagedInstances, type InstanceInfo } from '../aws';
import { SEED_ID_TAG_KEY, SEED_TAG_VALUE } from './identity';

/** Seed instances, optionally narrowed to one seed id. */
export function findSeedInstances(tagKey: string, seedId?: string): Promise<InstanceInfo[]> {
  return findManagedInstances(
    tagKey,
    SEED_TAG_VALUE,
    seedId ? [{ Name: `tag:${SEED_ID_TAG_KEY}`, Values: [seedId] }] : [],
  );
}
