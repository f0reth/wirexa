import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "../../../components/ui/resizable";
import styles from "./mqtt.module.css";
import { BrokerTopicsPanel } from "./panels/broker-topics-panel";
import { MessageDetail } from "./panels/message-detail";
import { MessagesPanel } from "./panels/messages-panel";
import { SubscriptionsPanel } from "./panels/subscriptions-panel";

export function SubscribeTab() {
  return (
    <ResizablePanelGroup direction="horizontal" class={styles.tabContent}>
      <ResizablePanel defaultSize={30} minSize={20}>
        <ResizablePanelGroup direction="vertical">
          <ResizablePanel defaultSize={55} minSize={25}>
            <SubscriptionsPanel />
          </ResizablePanel>
          <ResizableHandle withHandle />
          <ResizablePanel defaultSize={45} minSize={20}>
            <BrokerTopicsPanel />
          </ResizablePanel>
        </ResizablePanelGroup>
      </ResizablePanel>

      <ResizableHandle withHandle />

      <ResizablePanel defaultSize={70} minSize={40}>
        <ResizablePanelGroup direction="vertical">
          <ResizablePanel defaultSize={60} minSize={20}>
            <MessagesPanel />
          </ResizablePanel>
          <ResizableHandle withHandle />
          <ResizablePanel defaultSize={40} minSize={15}>
            <MessageDetail />
          </ResizablePanel>
        </ResizablePanelGroup>
      </ResizablePanel>
    </ResizablePanelGroup>
  );
}
