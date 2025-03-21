/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package org.apache.beam.sdk.metrics;

import java.util.List;
import java.util.Set;
import org.apache.beam.vendor.guava.v32_1_2_jre.com.google.common.collect.ImmutableList;
import org.apache.beam.vendor.guava.v32_1_2_jre.com.google.common.collect.ImmutableSet;
import org.junit.Assert;
import org.junit.Test;

public class BoundedTrieResultTest {

  @Test
  public void testCreate() {
    Set<List<String>> inputSet =
        ImmutableSet.of(
            ImmutableList.of("a", "b"), ImmutableList.of("c", "d"), ImmutableList.of("e"));
    BoundedTrieResult result = BoundedTrieResult.create(inputSet);
    Assert.assertEquals(inputSet, result.getResult());
  }

  @Test
  public void testCreate_empty() {
    Set<List<String>> inputSet = ImmutableSet.of();
    BoundedTrieResult result = BoundedTrieResult.create(inputSet);
    Assert.assertEquals(inputSet, result.getResult());
  }

  @Test
  public void testEmpty() {
    BoundedTrieResult result = BoundedTrieResult.empty();
    Assert.assertTrue(result.getResult().isEmpty());
  }

  @Test
  public void testImmutability() {
    Set<List<String>> inputSet =
        ImmutableSet.of(
            ImmutableList.of("a", "b"), ImmutableList.of("c", "d"), ImmutableList.of("e"));
    BoundedTrieResult result = BoundedTrieResult.create(inputSet);

    // Try to modify the set returned by getResult()
    try {
      result.getResult().add(ImmutableList.of("f"));
      Assert.fail("UnsupportedOperationException expected");
    } catch (UnsupportedOperationException expected) {
    }
  }
}
