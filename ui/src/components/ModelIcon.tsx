import { Icon } from "./Icon";

import { modelIdentity } from "../model-identity";
export { modelIdentity } from "../model-identity";

export function ModelIcon({
  id,
  provider,
  size = 24,
}: {
  id?: string;
  provider?: string;
  size?: number;
}) {
  const identity = modelIdentity(id, provider);
  return identity ? (
    <img
      class="model-icon"
      src={`/model-icons/${identity}.svg`}
      width={size}
      height={size}
      alt=""
    />
  ) : (
    <span
      class="model-icon model-icon-fallback"
      style={{ width: size, height: size }}
    >
      <Icon name="code" size={Math.max(14, size - 5)} />
    </span>
  );
}
