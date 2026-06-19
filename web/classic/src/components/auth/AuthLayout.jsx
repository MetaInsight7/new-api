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

import React, { useEffect, useLayoutEffect, useRef, useState } from 'react';
import './auth.css';

const clamp = (value, min, max) => Math.min(Math.max(value, min), max);

const CREATURES = [
  {
    key: 'violet',
    bodyClass: 'auth-creature-violet',
    eyesClass: 'auth-eyes-violet',
    speedClass: 'auth-eyes-slow',
    eyeType: 'white',
    baseLeft: 45,
    baseTop: 40,
    eyeSize: 18,
    pupilSize: 7,
    maxDistance: 5,
    blink: true,
  },
  {
    key: 'ink',
    bodyClass: 'auth-creature-ink',
    eyesClass: 'auth-eyes-ink',
    speedClass: 'auth-eyes-slow',
    eyeType: 'white',
    baseLeft: 26,
    baseTop: 32,
    eyeSize: 16,
    pupilSize: 6,
    maxDistance: 4,
    blink: true,
  },
  {
    key: 'coral',
    bodyClass: 'auth-creature-coral',
    eyesClass: 'auth-eyes-coral',
    speedClass: 'auth-eyes-fast',
    eyeType: 'solid',
    baseLeft: 82,
    baseTop: 90,
    eyeSize: 12,
    maxDistance: 5,
  },
  {
    key: 'sun',
    bodyClass: 'auth-creature-sun',
    eyesClass: 'auth-eyes-sun',
    speedClass: 'auth-eyes-fast',
    eyeType: 'solid',
    baseLeft: 52,
    baseTop: 40,
    eyeSize: 12,
    maxDistance: 5,
    mouth: true,
  },
];

const EYE_COUNT = 2;
const AUTH_POINTER_STORAGE_KEY = 'new-api-auth-pointer';
const RIGHT_ENTRY_POINTER = {
  x: Number.POSITIVE_INFINITY,
  y: Number.NaN,
  animateOnAuthEnter: true,
  synthetic: true,
};
let lastAuthPointer = null;

const getStoredAuthPointer = () => {
  if (typeof window === 'undefined') return null;

  if (window.__newApiAuthPointer || lastAuthPointer) {
    return window.__newApiAuthPointer || lastAuthPointer;
  }

  try {
    const savedPointer = window.sessionStorage.getItem(
      AUTH_POINTER_STORAGE_KEY,
    );
    if (!savedPointer) return null;

    const pointer = JSON.parse(savedPointer);
    if (!Number.isFinite(pointer?.x) || !Number.isFinite(pointer?.y)) {
      return null;
    }

    return {
      x: pointer.x,
      y: pointer.y,
      animateOnAuthEnter: true,
    };
  } catch (error) {
    return null;
  }
};

const setStoredAuthPointer = (pointer, options = {}) => {
  const nextPointer = { ...pointer, ...options };
  lastAuthPointer = nextPointer;
  if (typeof window !== 'undefined') {
    window.__newApiAuthPointer = nextPointer;

    if (
      !nextPointer.synthetic &&
      Number.isFinite(nextPointer.x) &&
      Number.isFinite(nextPointer.y)
    ) {
      try {
        window.sessionStorage.setItem(
          AUTH_POINTER_STORAGE_KEY,
          JSON.stringify({
            x: nextPointer.x,
            y: nextPointer.y,
          }),
        );
      } catch (error) {
        // Ignore storage failures; pointer memory is a visual enhancement only.
      }
    }
  }
};

const createMascotPose = ({
  faceX = 0,
  faceY = 0,
  bodySkew = 0,
  lookRight = false,
} = {}) => ({
  creatures: Object.fromEntries(
    CREATURES.map((creature) => [
      creature.key,
      {
        bodyStyle: { transform: `skewX(${bodySkew}deg)` },
        eyesStyle: {
          left: `${creature.baseLeft + faceX}px`,
          top: `${creature.baseTop + faceY}px`,
        },
        eyeStyles: Array.from({ length: EYE_COUNT }, () => ({
          transform: `translate(${lookRight ? creature.maxDistance : 0}px, 0px)`,
        })),
      },
    ]),
  ),
  mouthStyle: {
    left: `${40 + faceX}px`,
    top: `${88 + faceY}px`,
  },
});

const cloneMascotPose = (pose) => ({
  creatures: Object.fromEntries(
    CREATURES.map((creature) => {
      const creaturePose = pose.creatures[creature.key];

      return [
        creature.key,
        {
          bodyStyle: { ...creaturePose.bodyStyle },
          eyesStyle: { ...creaturePose.eyesStyle },
          eyeStyles: creaturePose.eyeStyles.map((style) => ({ ...style })),
        },
      ];
    }),
  ),
  mouthStyle: { ...pose.mouthStyle },
});

const ENTRY_MASCOT_POSE = createMascotPose();
let lastMascotPose = ENTRY_MASCOT_POSE;

const useRandomBlink = () => {
  const [isBlinking, setIsBlinking] = useState(false);

  useEffect(() => {
    let timeoutId;

    const scheduleBlink = () => {
      timeoutId = window.setTimeout(
        () => {
          setIsBlinking(true);
          timeoutId = window.setTimeout(() => {
            setIsBlinking(false);
            scheduleBlink();
          }, 150);
        },
        Math.random() * 4000 + 3000,
      );
    };

    scheduleBlink();

    return () => window.clearTimeout(timeoutId);
  }, []);

  return isBlinking;
};

const getFaceMotion = (element, pointer) => {
  if (!element || !pointer) {
    return { faceX: 0, faceY: 0, bodySkew: 0 };
  }

  const rect = element.getBoundingClientRect();
  const centerX = rect.left + rect.width / 2;
  const faceAnchorY = rect.top + rect.height / 3;
  const pointerX = Number.isFinite(pointer.x) ? pointer.x : centerX + 300;
  const pointerY = Number.isFinite(pointer.y) ? pointer.y : faceAnchorY;
  const deltaX = pointerX - centerX;

  return {
    faceX: clamp(deltaX / 20, -15, 15),
    faceY: clamp((pointerY - faceAnchorY) / 30, -10, 10),
    bodySkew: clamp(-deltaX / 120, -6, 6),
  };
};

const getEyeOffset = (element, pointer, maxDistance) => {
  if (!element || !pointer) {
    return { x: 0, y: 0 };
  }

  const rect = element.getBoundingClientRect();
  const centerX = rect.left + rect.width / 2;
  const centerY = rect.top + rect.height / 2;
  const pointerX = Number.isFinite(pointer.x) ? pointer.x : centerX + 300;
  const pointerY = Number.isFinite(pointer.y) ? pointer.y : centerY;
  const deltaX = pointerX - centerX;
  const deltaY = pointerY - centerY;
  const distance = Math.min(Math.sqrt(deltaX ** 2 + deltaY ** 2), maxDistance);
  const angle = Math.atan2(deltaY, deltaX);

  return {
    x: Math.cos(angle) * distance,
    y: Math.sin(angle) * distance,
  };
};

const getEyeTargets = (eyes, creature) => {
  if (creature.eyeType === 'white') {
    return Array.from(eyes.querySelectorAll('.auth-white-eye'))
      .map((eye) => ({
        anchor: eye,
        target: eye.querySelector('.auth-pupil'),
      }))
      .filter(({ target }) => target);
  }

  return Array.from(eyes.querySelectorAll('.auth-black-eye')).map((eye) => ({
    anchor: eye,
    target: eye,
  }));
};

const applyMascotMotion = (root, pointer) => {
  if (!root || !pointer) return;

  const nextPose = cloneMascotPose(lastMascotPose);
  let sunMotion = null;

  for (const creature of CREATURES) {
    const body = root.querySelector(`.${creature.bodyClass}`);
    const eyes = root.querySelector(`.${creature.eyesClass}`);

    if (!body || !eyes) return;

    const motion = getFaceMotion(body, pointer);
    const creaturePose = nextPose.creatures[creature.key];

    creaturePose.bodyStyle = {
      transform: `skewX(${motion.bodySkew || 0}deg)`,
    };
    creaturePose.eyesStyle = {
      left: `${creature.baseLeft + motion.faceX}px`,
      top: `${creature.baseTop + motion.faceY}px`,
    };

    Object.assign(body.style, creaturePose.bodyStyle);
    Object.assign(eyes.style, creaturePose.eyesStyle);

    getEyeTargets(eyes, creature).forEach(({ anchor, target }, index) => {
      const maxDistance = Number(
        anchor.dataset.maxDistance || creature.maxDistance,
      );
      const offset = getEyeOffset(anchor, pointer, maxDistance);
      const eyeStyle = {
        transform: `translate(${offset.x}px, ${offset.y}px)`,
      };

      target.style.transform = eyeStyle.transform;
      creaturePose.eyeStyles[index] = eyeStyle;
    });

    if (creature.mouth) {
      sunMotion = motion;
    }
  }

  if (sunMotion) {
    const mouth = root.querySelector('.auth-mouth');
    if (!mouth) return;

    nextPose.mouthStyle = {
      left: `${40 + sunMotion.faceX}px`,
      top: `${88 + sunMotion.faceY}px`,
    };
    Object.assign(mouth.style, nextPose.mouthStyle);
  }

  lastMascotPose = nextPose;
};

const MascotEye = ({ creature, isBlinking, motionStyle }) => {
  if (creature.eyeType === 'white') {
    return (
      <span
        className='auth-white-eye'
        data-max-distance={creature.maxDistance}
        style={{
          width: creature.eyeSize,
          height: isBlinking ? 2 : creature.eyeSize,
        }}
      >
        {!isBlinking && (
          <span
            className='auth-pupil'
            style={{
              width: creature.pupilSize,
              height: creature.pupilSize,
              ...motionStyle,
            }}
          />
        )}
      </span>
    );
  }

  return (
    <span
      className='auth-black-eye'
      data-max-distance={creature.maxDistance}
      style={{
        width: creature.eyeSize,
        height: creature.eyeSize,
        ...motionStyle,
      }}
    />
  );
};

const MascotCreature = ({ creature, pose, isBlinking }) => {
  const creaturePose = pose.creatures[creature.key];

  return (
    <div
      className={`auth-creature ${creature.bodyClass}`}
      style={creaturePose.bodyStyle}
    >
      <div
        className={`auth-eyes ${creature.eyesClass} ${creature.speedClass}`}
        style={creaturePose.eyesStyle}
      >
        {Array.from({ length: EYE_COUNT }, (_, index) => (
          <MascotEye
            key={index}
            creature={creature}
            isBlinking={isBlinking}
            motionStyle={creaturePose.eyeStyles[index]}
          />
        ))}
      </div>

      {creature.mouth && (
        <span className='auth-mouth' style={pose.mouthStyle} />
      )}
    </div>
  );
};

const AuthMascot = () => {
  const mascotRef = useRef(null);
  const frameRef = useRef(null);
  const pointerRef = useRef(null);
  const initialPointer = getStoredAuthPointer() || RIGHT_ENTRY_POINTER;
  lastAuthPointer = initialPointer;
  const shouldAnimateOnEnter = Boolean(initialPointer?.animateOnAuthEnter);
  const pose = shouldAnimateOnEnter ? ENTRY_MASCOT_POSE : lastMascotPose;
  const blinking = {
    violet: useRandomBlink(),
    ink: useRandomBlink(),
  };

  useLayoutEffect(() => {
    const root = mascotRef.current;
    let firstFrame = null;
    let secondFrame = null;

    if (!window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      const storedPointer = getStoredAuthPointer() || RIGHT_ENTRY_POINTER;
      const shouldAnimateInitialMotion = Boolean(
        storedPointer?.animateOnAuthEnter,
      );

      pointerRef.current = storedPointer;

      if (!shouldAnimateInitialMotion) {
        root?.classList.add('auth-mascot-no-transition');
      }

      firstFrame = window.requestAnimationFrame(() => {
        if (!shouldAnimateInitialMotion && storedPointer) {
          setStoredAuthPointer(storedPointer, {
            animateOnAuthEnter: false,
          });
          applyMascotMotion(root, storedPointer);
        }

        secondFrame = window.requestAnimationFrame(() => {
          if (shouldAnimateInitialMotion && storedPointer) {
            setStoredAuthPointer(storedPointer, {
              animateOnAuthEnter: false,
            });
            applyMascotMotion(root, storedPointer);
          }

          root?.classList.remove('auth-mascot-no-transition');
        });
      });
    }

    return () => {
      if (firstFrame) {
        window.cancelAnimationFrame(firstFrame);
      }
      if (secondFrame) {
        window.cancelAnimationFrame(secondFrame);
      }
    };
  }, []);

  useEffect(() => {
    const reduceMotionQuery = window.matchMedia(
      '(prefers-reduced-motion: reduce)',
    );

    const applyMotion = () => {
      frameRef.current = null;

      const root = mascotRef.current;
      const pointer = pointerRef.current;
      if (!root || !pointer || reduceMotionQuery.matches) return;
      applyMascotMotion(root, pointer);
    };

    const handleMouseMove = (event) => {
      const pointer = { x: event.clientX, y: event.clientY };
      setStoredAuthPointer(pointer);
      pointerRef.current = pointer;

      if (frameRef.current || reduceMotionQuery.matches) return;

      frameRef.current = window.requestAnimationFrame(applyMotion);
    };

    window.addEventListener('mousemove', handleMouseMove, { passive: true });

    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
      if (frameRef.current) {
        window.cancelAnimationFrame(frameRef.current);
      }
    };
  }, []);

  return (
    <div ref={mascotRef} className='auth-mascot' aria-hidden='true'>
      {CREATURES.map((creature) => (
        <MascotCreature
          key={creature.key}
          creature={creature}
          pose={pose}
          isBlinking={Boolean(blinking[creature.key])}
        />
      ))}
    </div>
  );
};

const AuthLayout = ({ children }) => {
  return (
    <div className='classic-page-fill auth-page'>
      <section className='auth-visual-panel'>
        <AuthMascot />
      </section>
      <section className='auth-form-panel'>{children}</section>
    </div>
  );
};

export default AuthLayout;
