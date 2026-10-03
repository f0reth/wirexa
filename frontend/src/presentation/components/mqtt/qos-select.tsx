import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../../components/ui/select";
import { isQos, type Qos } from "../../../domain/mqtt/types";
import styles from "./mqtt.module.css";

interface QosSelectProps {
  value: Qos;
  onChange: (qos: Qos) => void;
}

export function QosSelect(props: QosSelectProps) {
  return (
    <Select
      value={props.value.toString()}
      onValueChange={(v) => {
        const qos = parseInt(v, 10);
        if (isQos(qos)) props.onChange(qos);
      }}
    >
      <SelectTrigger class={styles.qosSelect}>
        <SelectValue placeholder="QoS" />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="0">QoS 0</SelectItem>
        <SelectItem value="1">QoS 1</SelectItem>
        <SelectItem value="2">QoS 2</SelectItem>
      </SelectContent>
    </Select>
  );
}
