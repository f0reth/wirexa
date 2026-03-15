import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../ui/select";
import styles from "./mqtt.module.css";

interface QosSelectProps {
  value: number;
  onChange: (qos: number) => void;
}

export function QosSelect(props: QosSelectProps) {
  return (
    <Select
      value={props.value.toString()}
      onValueChange={(v) => props.onChange(parseInt(v, 10))}
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
