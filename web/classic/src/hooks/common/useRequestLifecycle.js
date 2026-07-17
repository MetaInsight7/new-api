/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import { useCallback, useEffect, useRef } from 'react';

/**
 * Keeps async callbacks from updating state after unmount or after a newer
 * request for the same resource has completed.
 */
export const useRequestLifecycle = () => {
  const mountedRef = useRef(true);
  const sequenceRef = useRef(new Map());

  useEffect(() => {
    mountedRef.current = true;
    const sequence = sequenceRef.current;
    return () => {
      mountedRef.current = false;
      sequence.clear();
    };
  }, []);

  const beginRequest = useCallback((key = 'default') => {
    const nextSequence = (sequenceRef.current.get(key) || 0) + 1;
    sequenceRef.current.set(key, nextSequence);
    return nextSequence;
  }, []);

  const isCurrentRequest = useCallback((key, sequence) => {
    return mountedRef.current && sequenceRef.current.get(key) === sequence;
  }, []);

  const isMounted = useCallback(() => mountedRef.current, []);

  return { beginRequest, isCurrentRequest, isMounted };
};

export default useRequestLifecycle;
