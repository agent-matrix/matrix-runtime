import {
  Box, Flex, Group, Stack,
} from '@mantine/core'

import { AllModelList } from '@/features/models/components/AllModelList'
import { HotModelList } from '@/features/models/components/HotModelList'
import { ModelsFilterPanel } from '@/features/models/components/ModelsFilterPanel'

export function ModelsPage() {
  return (
    <Flex mih="100%" justify="center" pt="lg" pb="xl">
      <Group
        w="100%"
        gap="xl"
        align="stretch"
        wrap="nowrap"
      >
        <Flex
          flex={24}
          wrap="nowrap"
          gap="xl"
          miw={0}
        >
          {/* TODO: 1760px parent width cannot support maxWidth 400 */}
          <Box
            flex={1}
            miw={260}
            maw={400}
          >
            <ModelsFilterPanel />
          </Box>

          <Box
            mt="sm"
            style={{
              borderInlineEnd: '1px solid var(--mantine-color-gray-3)',
            }}
          />
        </Flex>

        <Stack
          flex={76}
          gap="lg"
        >
          <HotModelList />

          <AllModelList />
        </Stack>
      </Group>
    </Flex>
  )
}
