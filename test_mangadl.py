import unittest
import gevent
import random
import mangadl

class test_Gevent_queue(unittest.TestCase):
    def worker_nodelay(self, task):
        gevent.sleep(0)
        return (task, True)

    def worker_delay(self, task):
        gevent.sleep(random.random() * 2)
        return (task, True)

    def test_queue_sorted(self):
        ''' should return finished tasks in sorted order '''
        tasks = range(4)
        q = mangadl.Gevent_queue(tasks, self.worker_delay)
        res = q.execute()
        expected = [
            (0, True),
            (1, True),
            (2, True),
            (3, True)
        ]
        random.shuffle(expected)
        self.assertEqual(res, sorted(expected))

    def test_queue_4(self):
        ''' should finish 4 tasks '''
        tasks = range(4)
        q = mangadl.Gevent_queue(tasks, self.worker_nodelay)
        res = q.execute()
        expected = [
            (0, True),
            (1, True),
            (2, True),
            (3, True)
        ]
        self.assertEqual(res, expected)

    def test_queue_10(self):
        ''' should finish 10 tasks '''
        tasks = range(10)
        q = mangadl.Gevent_queue(tasks, self.worker_nodelay)
        res = q.execute()
        expected = [ (i, True) for i in range(10) ]
        self.assertEqual(res, expected)

    def test_queue_100(self):
        ''' should finish 100 tasks '''
        tasks = range(100)
        q = mangadl.Gevent_queue(tasks, self.worker_nodelay)
        res = q.execute()
        expected = [ (i, True) for i in range(100) ]
        self.assertEqual(res, expected)